# OpenList Driver Manager

OpenList Driver Manager 是一个独立的服务，用于管理和隔离驱动程序，实现驱动与主程序的分离。

## 架构概述

新的架构将驱动程序从主程序中分离出来：

```
网盘 => 驱动（网盘基本功能+认证的标准化）=> wrap（适配文件系统）=> 文件系统 => 用户界面
```

- **主程序（OpenList）**: 启动驱动管理器服务器，等待驱动管理器连接
- **驱动管理器**: 独立进程，主动连接到OpenList并管理所有驱动实例
- **流式传输**: 驱动管理器主动连接OpenList，建立TCP流式通信

## 功能特性

- ✅ 驱动隔离：驱动运行在独立进程中，提高稳定性
- ✅ 流式传输：通过TCP协议进行高效通信
- ✅ 多管理器支持：支持连接多个驱动管理器
- ✅ 国际化支持：支持 en_us 和 zh_cn
- ✅ 热插拔：支持动态添加/移除驱动实例
- ✅ 兼容性：保持现有文件处理模式不变

## 快速开始

### 1. 启动驱动管理器

#### Linux/macOS
```bash
cd driver-manager
chmod +x start.sh
./start.sh
```

#### Windows
```cmd
cd driver-manager
start.bat
```

#### 手动启动
```bash
cd driver-manager
go build -o driver-manager main.go
./driver-manager -openlist-host=localhost -openlist-port=5245 -manager-id=dm-001
```

### 2. 启动主程序

主程序会自动启动驱动管理器服务器，等待驱动管理器连接：

```bash
# 启动OpenList主程序（自动启动驱动管理器服务器在5245端口）
./openlist server
```

### 3. 验证连接

启动主程序后，可以通过以下API验证驱动管理器连接：

```bash
# 获取所有驱动信息（包括远程驱动）
curl http://localhost:5244/api/admin/driver/list

# 获取驱动管理器信息
curl http://localhost:5244/api/admin/driver/manager_info
```

## 通信协议

**注意**: 驱动管理器不再需要配置文件，所有驱动配置都从OpenList数据库中读取。

驱动管理器使用基于JSON的消息协议：

### 握手消息
```json
{
  "id": "handshake",
  "type": "handshake",
  "result": {
    "driver_count": 50,
    "drivers": {
      "Local": {
        "name": "Local",
        "config": {...},
        "items": [...],
        "i18n": {
          "en_us": {...},
          "zh_cn": {...}
        }
      }
    }
  }
}
```

### 请求消息
```json
{
  "id": "req-123",
  "type": "request",
  "method": "list_drivers",
  "params": {}
}
```

### 响应消息
```json
{
  "id": "req-123",
  "type": "response",
  "result": {...}
}
```

## API 方法

### 驱动管理
- `list_drivers`: 列出所有可用驱动
- `get_driver_info`: 获取特定驱动信息
- `create_instance`: 创建驱动实例
- `remove_instance`: 移除驱动实例
- `list_instances`: 列出所有驱动实例
- `enable_instance`: 启用驱动实例
- `disable_instance`: 禁用驱动实例

### 驱动操作
- `execute_operation`: 执行驱动操作
  - `list`: 列出文件
  - `link`: 获取下载链接
  - `get`: 获取文件信息
  - `other`: 其他操作

## 国际化支持

每个驱动都包含 i18n 配置：

```json
{
  "i18n": {
    "en_us": {
      "driver_name": "Local Storage",
      "root_folder_path": "Root Folder Path",
      "root_folder_path_help": "The root folder path for local storage"
    },
    "zh_cn": {
      "driver_name": "本地存储",
      "root_folder_path": "根文件夹路径",
      "root_folder_path_help": "本地存储的根文件夹路径"
    }
  }
}
```

## 故障排除

### ��接问题
1. 确保驱动管理器正在运行
2. 检查端口是否被占用
3. 验证防火墙设置

### 驱动问题
1. 检查驱动配置是否正确
2. 查看驱动管理器日志
3. 验证驱动依赖是否满足

## 开发指南

### 添加新驱动
1. 在 `drivers/` 目录下实现驱动
2. 确保驱动实现了必要的接口
3. 在驱动管理器中注册驱动

### 扩展协议
1. 在 `protocol/protocol.go` 中添加新方法
2. 在客户端添加对应的调用方法
3. 更新API文档

## 性能优化

- 使用连接池管理多个驱动管理器连接
- 实现请求缓存减少重复调用
- 优化序列化/反序列化性能
- 使用压缩减少网络传输

## 安全考虑

- 实现认证机制
- 加密敏感数据传输
- 限制访问权限
- 审计日志记录