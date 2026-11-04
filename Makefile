.PHONY: all build build-frontend build-backend clean run dev help

# =============================================================================
# 构建产物说明：
#   前端源码         web/src/
#   前端构建产物     web/dist/        ← Vite 默认 outDir
#   Go embed 目录    internal/web/dist/ ← Makefile 从 web/dist 拷贝过来
#   最终二进制       goldapadmin       ← go build，自动 embed internal/web/dist
#
# 职责分离：Vite 只负责 web/dist，Makefile 负责拷贝 + 编译 Go
# =============================================================================

FRONTEND_DIST := web/dist
EMBED_DIR     := internal/web/dist
BIN           := goldapadmin

# 仅构建前端到 web/dist（不含拷贝）
build-frontend:
	@echo "==> [1/3] Building frontend -> $(FRONTEND_DIST)"
	cd web && npm install --silent && npm run build
	@echo "    Frontend built OK: $(FRONTEND_DIST)"

# 把前端产物拷贝到 Go embed 目录
sync-frontend: build-frontend
	@echo "==> [2/3] Syncing $(FRONTEND_DIST) -> $(EMBED_DIR)"
	rm -rf $(EMBED_DIR)
	mkdir -p $(dir $(EMBED_DIR))
	cp -R $(FRONTEND_DIST)/. $(EMBED_DIR)/
	@echo "    Synced OK: $(EMBED_DIR)"

# 编译 Go 二进制（自动嵌入 internal/web/dist）
build-backend: sync-frontend
	@echo "==> [3/3] Compiling Go binary -> $(BIN)"
	go build -o $(BIN) ./cmd/server
	@echo "===== Done ====="
	@ls -lh $(BIN)

# 完整构建 = 前端 + 拷贝 + 后端（单二进制）
build: build-backend
all: build

# 仅快速编译后端（跳过前端重构建和拷贝，适用于只改了 Go 代码）
# 前提：internal/web/dist 已是最新
build-go:
	go build -o $(BIN) ./cmd/server

dev: build-go
	./$(BIN)

run:
	./$(BIN)

clean:
	rm -f $(BIN)
	rm -rf $(FRONTEND_DIST)
	rm -rf $(EMBED_DIR)
	rm -rf data/
	@echo "Cleaned."

help:
	@echo "Targets:"
	@echo "  make build            完整构建（前端 dist -> 拷贝 -> Go 单二进制）"
	@echo "  make build-frontend   仅构建前端到 web/dist"
	@echo "  make sync-frontend    前端构建 + 拷贝到 internal/web/dist"
	@echo "  make build-backend    完整构建（同 make build）"
	@echo "  make build-go         仅编译 Go（跳过前端，Go 代码改动快编）"
	@echo "  make dev              编译并运行（跳过前端）"
	@echo "  make run              运行已有 $(BIN)"
	@echo "  make clean            清理所有产物"
	@echo "  make help             本帮助"
