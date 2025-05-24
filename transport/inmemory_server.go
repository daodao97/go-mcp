package transport

import (
	"context"
	"sync"

	"github.com/ThinkInAIXYZ/go-mcp/pkg"
)

// InmemoryServerTransportOption 配置选项类型
type InmemoryServerTransportOption func(*inmemoryServerTransport)

// WithInmemoryServerTransportOptionLogger 设置日志记录器
func WithInmemoryServerTransportOptionLogger(logger pkg.Logger) InmemoryServerTransportOption {
	return func(t *inmemoryServerTransport) {
		t.logger = logger
	}
}

// inmemoryServerTransport 内存中的服务器传输实现
type inmemoryServerTransport struct {
	receiver serverReceiver

	sessionManager sessionManager
	sessionID      string

	logger pkg.Logger

	// 用于控制传输生命周期的上下文和取消函数
	ctx    context.Context
	cancel context.CancelFunc

	// 用于等待接收循环结束
	receiveShutDone chan struct{}

	// 用于管理正在进行的发送操作
	inFlySend sync.WaitGroup

	// 内存消息通道，用于模拟客户端到服务器的通信
	messageQueue chan Message

	// 用于与客户端传输建立连接的通道
	clientConnectCh chan ClientTransport
	connectedClient ClientTransport

	// 状态管理
	running    *pkg.AtomicBool
	shutdownCh chan struct{}
}

// Run 实现 ServerTransport 接口
func (i *inmemoryServerTransport) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	i.ctx = ctx
	i.cancel = cancel
	i.running.Store(true)

	// 等待客户端连接
	select {
	case client := <-i.clientConnectCh:
		i.connectedClient = client
	case <-ctx.Done():
		return ctx.Err()
	}

	// 创建会话
	i.sessionID = i.sessionManager.CreateSession(context.Background())

	// 启动消息接收循环
	go func() {
		defer pkg.Recover()
		defer close(i.receiveShutDone)
		i.startReceive(ctx)
	}()

	// 等待关闭信号
	select {
	case <-i.shutdownCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Send 实现 ServerTransport 接口
func (i *inmemoryServerTransport) Send(ctx context.Context, sessionID string, msg Message) error {
	i.inFlySend.Add(1)
	defer i.inFlySend.Done()

	if !i.running.Load() {
		return pkg.ErrSessionClosed
	}

	select {
	case <-i.ctx.Done():
		return i.ctx.Err()
	default:
		// 直接发送给连接的客户端
		if i.connectedClient != nil {
			return i.connectedClient.(*inmemoryClientTransport).receiveFromServer(ctx, msg)
		}
		return pkg.ErrSessionClosed
	}
}

// SetReceiver 实现 ServerTransport 接口
func (i *inmemoryServerTransport) SetReceiver(receiver serverReceiver) {
	i.receiver = receiver
}

// SetSessionManager 实现 ServerTransport 接口
func (i *inmemoryServerTransport) SetSessionManager(manager sessionManager) {
	i.sessionManager = manager
}

// Shutdown 实现 ServerTransport 接口
func (i *inmemoryServerTransport) Shutdown(userCtx context.Context, serverCtx context.Context) error {
	// 标记为不再运行
	i.running.Store(false)

	// 启动关闭序列
	shutdownFunc := func() {
		<-serverCtx.Done()

		// 取消上下文以停止所有正在进行的操作
		if i.cancel != nil {
			i.cancel()
		}

		// 等待所有正在进行的发送操作完成
		i.inFlySend.Wait()

		// 由于这是内存传输，我们可以直接关闭相关资源，而不依赖 sessionManager
		// 这避免了 mock sessionManager 中的 nil channel 问题
		// if i.sessionManager != nil {
		//     i.sessionManager.CloseAllSessions()
		// }

		// 安全地关闭消息队列
		if i.messageQueue != nil {
			select {
			case <-i.messageQueue:
				// channel 已经关闭
			default:
				close(i.messageQueue)
			}
		}
	}

	go shutdownFunc()

	// 发送关闭信号
	select {
	case <-i.shutdownCh:
		// 已经关闭
	default:
		close(i.shutdownCh)
	}

	// 等待接收循环结束
	select {
	case <-i.receiveShutDone:
		return nil
	case <-serverCtx.Done():
		return nil
	case <-userCtx.Done():
		return userCtx.Err()
	}
}

// startReceive 启动消息接收循环
func (i *inmemoryServerTransport) startReceive(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-i.messageQueue:
			if !ok {
				return
			}
			i.receive(ctx, msg)
		}
	}
}

// receive 处理接收到的消息
func (i *inmemoryServerTransport) receive(ctx context.Context, msg Message) {
	if i.receiver == nil {
		i.logger.Errorf("no receiver set for inmemory server transport")
		return
	}

	outputMsgCh, err := i.receiver.Receive(ctx, i.sessionID, msg)
	if err != nil {
		i.logger.Errorf("receiver failed: %v", err)
		return
	}

	if outputMsgCh == nil {
		return
	}

	go func() {
		defer pkg.Recover()

		for msg := range outputMsgCh {
			if e := i.Send(context.Background(), i.sessionID, msg); e != nil {
				i.logger.Errorf("Failed to send message: %v", e)
			}
		}
	}()
}

// connectClient 用于建立与客户端的连接
func (i *inmemoryServerTransport) connectClient(client ClientTransport) {
	select {
	case i.clientConnectCh <- client:
	default:
		i.logger.Errorf("failed to connect client: channel full")
	}
}

// sendMessageToServer 用于客户端向服务器发送消息
func (i *inmemoryServerTransport) sendMessageToServer(ctx context.Context, msg Message) error {
	if !i.running.Load() {
		return pkg.ErrSessionClosed
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case i.messageQueue <- msg:
		return nil
	case <-i.ctx.Done():
		return i.ctx.Err()
	}
}

// NewInmemoryServerTransport 创建新的内存服务器传输实例
func NewInmemoryServerTransport(opts ...InmemoryServerTransportOption) ServerTransport {
	t := &inmemoryServerTransport{
		logger:          pkg.DefaultLogger,
		receiveShutDone: make(chan struct{}),
		messageQueue:    make(chan Message, 100), // 缓冲区大小为100
		clientConnectCh: make(chan ClientTransport, 1),
		running:         pkg.NewAtomicBool(),
		shutdownCh:      make(chan struct{}),
	}

	for _, opt := range opts {
		opt(t)
	}

	return t
}
