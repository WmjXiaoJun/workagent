# 运行环境固化 —— 保证跨设备可复现
# 注意：仅有 Dockerfile 不足以支撑"结论可复现"。
# 完整的可复现性还需要 attest/closure.json 中登记的计算闭包摘要。

# ---- 阶段一：构建 CLI（静态二进制） ----
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
RUN CGO_ENABLED=0 go build -trimpath -o /out/workagent ./cmd/workagent

# ---- 阶段二：运行环境 ----
# 基础镜像与执行体保持一致（示例执行体是 Python；换成你自己的语言即可）
FROM python:3.11-slim

LABEL org.opencontainers.image.title="workagent" \
      org.opencontainers.image.version="0.3.0" \
      org.opencontainers.image.licenses="MIT"

ENV PYTHONHASHSEED=0 \
    PYTHONDONTWRITEBYTECODE=1 \
    LC_ALL=C.UTF-8 \
    LANG=C.UTF-8 \
    TZ=UTC

COPY --from=build /out/workagent /usr/local/bin/workagent

WORKDIR /workagent
COPY . /workagent

CMD ["workagent", "run", "--input", "examples/bid_quote/input.json", "."]
