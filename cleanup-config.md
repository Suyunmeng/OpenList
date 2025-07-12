# 清理配置文件

请手动删除以下文件，因为它们不再需要：

```bash
rm driver-manager/driver-config.json
```

或者在Windows中：

```cmd
del driver-manager\driver-config.json
```

这个文件已经不再需要，因为所有驱动配置现在都从OpenList数据库中读取。