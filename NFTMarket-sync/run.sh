nohup go run main.go daemon -c "./config/config_import.toml" > logs/sync_service.log 2>&1 &

# 获取进程ID
echo $! > logs/sync_service.pid

# 显示进程状态
echo "同步服务已启动，进程ID: $(cat logs/sync_service.pid)"

# 上一个区块的 hash 和 当前区块的 parentHash 比较