package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ThinkInAIXYZ/go-mcp/client"
	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"github.com/ThinkInAIXYZ/go-mcp/server"
	"github.com/ThinkInAIXYZ/go-mcp/transport"
)

type echoRequest struct {
	Message string `json:"message" description:"要回显的消息"`
}

func main() {
	// 创建配对的内存传输
	serverTransport, clientTransport := transport.NewInmemoryTransportPair(nil, nil)

	// 创建 MCP 服务器
	mcpServer, err := server.NewServer(serverTransport,
		server.WithServerInfo(protocol.Implementation{
			Name:    "inmemory-example",
			Version: "1.0.0",
		}),
	)
	if err != nil {
		log.Fatalf("创建服务器失败: %v", err)
	}

	// 注册一个简单的 echo 工具
	tool, err := protocol.NewTool("echo", "回显输入的消息", echoRequest{})
	if err != nil {
		log.Fatalf("创建工具失败: %v", err)
	}

	mcpServer.RegisterTool(tool, func(ctx context.Context, request *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
		var req echoRequest
		if err := protocol.VerifyAndUnmarshal(request.RawArguments, &req); err != nil {
			return nil, err
		}

		return &protocol.CallToolResult{
			Content: []protocol.Content{
				&protocol.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Echo: %s", req.Message),
				},
			},
		}, nil
	})

	// 启动服务器
	go func() {
		if err := mcpServer.Run(); err != nil {
			log.Printf("服务器运行错误: %v", err)
		}
	}()

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)

	// 创建 MCP 客户端
	mcpClient, err := client.NewClient(clientTransport,
		client.WithClientInfo(protocol.Implementation{
			Name:    "inmemory-client",
			Version: "1.0.0",
		}),
	)
	if err != nil {
		log.Fatalf("创建客户端失败: %v", err)
	}

	defer mcpClient.Close()

	// 使用客户端调用工具
	ctx := context.Background()

	// 列出可用工具
	tools, err := mcpClient.ListTools(ctx)
	if err != nil {
		log.Fatalf("列出工具失败: %v", err)
	}

	fmt.Printf("可用工具: %d 个\n", len(tools.Tools))
	for _, tool := range tools.Tools {
		fmt.Printf("- %s: %s\n", tool.Name, tool.Description)
	}

	// 调用 echo 工具
	result, err := mcpClient.CallTool(ctx, &protocol.CallToolRequest{
		Name: "echo",
		Arguments: map[string]interface{}{
			"message": "Hello, inmemory transport!",
		},
	})
	if err != nil {
		log.Fatalf("调用工具失败: %v", err)
	}

	fmt.Printf("工具调用结果:\n")
	for i, content := range result.Content {
		if textContent, ok := content.(*protocol.TextContent); ok {
			fmt.Printf("[%d] %s\n", i+1, textContent.Text)
		}
	}

	fmt.Println("inmemory transport 示例运行成功！")
}
