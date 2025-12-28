.PHONY: run build gen tidy

# 运行服务
run:
	go run cmd/server/main.go

# 构建
build:
	go build -o bin/admin cmd/server/main.go

# 生成数据库模型代码
gen:
	go run cmd/gen/main.go

# 整理依赖
tidy:
	go mod tidy

# 清理
clean:
	rm -rf bin/

# 帮助
help:
	@echo "可用命令:"
	@echo "  make run   - 运行服务"
	@echo "  make build - 构建可执行文件"
	@echo "  make gen   - 从数据库生成模型代码"
	@echo "  make tidy  - 整理依赖"
	@echo "  make clean - 清理构建文件"

