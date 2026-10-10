# 运行环境固化 —— 保证跨设备可复现
# 注意：仅有 Dockerfile 不足以支撑"结论可复现"。
# 完整的可复现性还需要 attest/closure.json 中登记的计算闭包摘要。
FROM python:3.11-slim

LABEL org.opencontainers.image.title="workagent" \
      org.opencontainers.image.version="0.3.0" \
      org.opencontainers.image.licenses="MIT"

ENV PYTHONHASHSEED=0 \
    PYTHONDONTWRITEBYTECODE=1 \
    LC_ALL=C.UTF-8 \
    LANG=C.UTF-8 \
    TZ=UTC

WORKDIR /workagent
COPY . /workagent

# 零依赖：本项目仅使用 Python 标准库
CMD ["python", "tools/workagent.py", "run", "--input", "examples/bid_quote/input.json"]
