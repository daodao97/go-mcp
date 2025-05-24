# InMemory Transport 示例

这个示例演示了如何使用 Go MCP SDK 的 inmemory transport 实现。

## 什么是 InMemory Transport？

InMemory transport 是一种 MCP（Model Context Protocol）传输实现，它在同一进程的内存中直接连接客户端和服务器，无需网络通信或外部进程。这种传输方式特别适用于：

- **单元测试和集成测试**：可以快速创建客户端-服务器对进行测试
- **本地开发和调试**：方便在同一进程中测试 MCP 功能
- **嵌入式应用**：当你需要在同一应用程序中同时运行 MCP 客户端和服务器时
- **基准测试**：消除网络延迟，专注于测试 MCP 协议本身的性能

## 核心特性

- **零延迟**：内存中直接通信，没有网络或序列化开销
- **简单易用**：一个函数调用即可创建配对的客户端和服务器传输
- **完全兼容**：实现了完整的 MCP ServerTransport 和 ClientTransport 接口
- **线程安全**：支持并发操作和优雅关闭

## 使用方法

### 基本用法

```go
// 创建配对的传输
serverTransport, clientTransport := transport.NewInmemoryTransportPair(nil, nil)

// 创建服务器
server, _ := server.NewServer(serverTransport, /* options */)

// 创建客户端  
client, _ := client.NewClient(clientTransport, /* options */)
```

### 高级配置

```go
// 自定义日志记录器
serverOpts := []transport.InmemoryServerTransportOption{
    transport.WithInmemoryServerTransportOptionLogger(customLogger),
}

clientOpts := []transport.InmemoryClientTransportOption{
    transport.WithInmemoryClientTransportOptionLogger(customLogger),
}

serverTransport, clientTransport := transport.NewInmemoryTransportPair(serverOpts, clientOpts)
```

## 运行示例

```bash
cd examples/inmemory
go run main.go
```

## 输出示例

```
可用工具: 1 个
- echo: 回显输入的消息
工具调用结果:
[1] Echo: Hello, inmemory transport!
inmemory transport 示例运行成功！
```

## 与其他传输方式的对比

| 传输方式 | 用途 | 优点 | 缺点 |
|---------|-----|------|------|
| InMemory | 测试、开发、嵌入式 | 零延迟、简单 | 仅限同进程 |
| Stdio | 命令行工具 | 简单、广泛支持 | 需要外部进程 |
| HTTP/SSE | 远程服务 | 支持远程、Web友好 | 网络延迟、复杂性 |

## 技术实现

InMemory transport 使用 Go channels 在客户端和服务器之间直接传递消息，避免了 JSON 序列化和网络通信的开销。它完全实现了 MCP 协议的语义，包括：

- 正确的初始化握手
- 会话管理
- 消息路由
- 优雅关闭

这使得它成为了解 MCP 协议和开发 MCP 应用程序的理想选择。 