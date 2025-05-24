package transport

import (
	"context"
	"sync"

	"github.com/ThinkInAIXYZ/go-mcp/pkg"
)

// InmemoryClientTransportOption 配置选项类型
type InmemoryClientTransportOption func(*inmemoryClientTransport)

// WithInmemoryClientTransportOptionLogger 设置日志记录器
func WithInmemoryClientTransportOptionLogger(logger pkg.Logger) InmemoryClientTransportOption {
	return func(t *inmemoryClientTransport) {
		t.logger = logger
	}
}

// inmemoryClientTransport 内存中的客户端传输实现
type inmemoryClientTransport struct {
	receiver clientReceiver

	logger pkg.Logger

	// 用于控制传输生命周期的上下文和取消函数
	ctx    context.Context
	cancel context.CancelFunc

	// 关联的服务器传输
	serverTransport *inmemoryServerTransport

	// 用于接收来自服务器的消息
	serverMessageQueue chan Message

	// 状态管理
	running *pkg.AtomicBool

	// 用于等待goroutine结束
	wg sync.WaitGroup
}

// Start 实现 ClientTransport 接口
func (i *inmemoryClientTransport) Start() error {
	ctx, cancel := context.WithCancel(context.Background())
	i.ctx = ctx
	i.cancel = cancel
	i.running.Store(true)

	// 连接到服务器
	if i.serverTransport != nil {
		i.serverTransport.connectClient(i)
	}

	// 启动消息接收循环
	i.wg.Add(1)
	go func() {
		defer pkg.Recover()
		defer i.wg.Done()
		i.startReceive(ctx)
	}()

	return nil
}

// Send 实现 ClientTransport 接口
func (i *inmemoryClientTransport) Send(ctx context.Context, msg Message) error {
	if !i.running.Load() {
		return pkg.ErrSessionClosed
	}

	if i.serverTransport != nil {
		return i.serverTransport.sendMessageToServer(ctx, msg)
	}

	return pkg.ErrSessionClosed
}

// SetReceiver 实现 ClientTransport 接口
func (i *inmemoryClientTransport) SetReceiver(receiver clientReceiver) {
	i.receiver = receiver
}

// Close 实现 ClientTransport 接口
func (i *inmemoryClientTransport) Close() error {
	i.running.Store(false)

	if i.cancel != nil {
		i.cancel()
	}

	// 关闭消息队列
	close(i.serverMessageQueue)

	// 等待所有goroutine结束
	i.wg.Wait()

	return nil
}

// startReceive 启动消息接收循环
func (i *inmemoryClientTransport) startReceive(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-i.serverMessageQueue:
			if !ok {
				return
			}
			if i.receiver != nil {
				if err := i.receiver.Receive(ctx, msg); err != nil {
					i.logger.Errorf("receiver failed: %v", err)
				}
			}
		}
	}
}

// receiveFromServer 用于从服务器接收消息
func (i *inmemoryClientTransport) receiveFromServer(ctx context.Context, msg Message) error {
	if !i.running.Load() {
		return pkg.ErrSessionClosed
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case i.serverMessageQueue <- msg:
		return nil
	case <-i.ctx.Done():
		return i.ctx.Err()
	}
}

// NewInmemoryClientTransport 创建新的内存客户端传输实例
func NewInmemoryClientTransport(serverTransport ServerTransport, opts ...InmemoryClientTransportOption) ClientTransport {
	st, ok := serverTransport.(*inmemoryServerTransport)
	if !ok {
		return nil
	}

	t := &inmemoryClientTransport{
		logger:             pkg.DefaultLogger,
		serverTransport:    st,
		serverMessageQueue: make(chan Message, 100), // 缓冲区大小为100
		running:            pkg.NewAtomicBool(),
	}

	for _, opt := range opts {
		opt(t)
	}

	return t
}

// NewInmemoryTransportPair 创建配对的内存传输实例
func NewInmemoryTransportPair(serverOpts []InmemoryServerTransportOption, clientOpts []InmemoryClientTransportOption) (ServerTransport, ClientTransport) {
	server := NewInmemoryServerTransport(serverOpts...)
	client := NewInmemoryClientTransport(server, clientOpts...)
	return server, client
}
