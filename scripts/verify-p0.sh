#!/bin/bash
# syskit P0 本地验证脚本
# 用法: ./scripts/verify-p0.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

echo "==> 1/4 运行全量测试"
go test ./...

echo "==> 2/4 编译六个正式支持目标"
"$SCRIPT_DIR/build.sh" all

echo "==> 3/4 构造临时配置与数据目录"
export SYSKIT_DATA_DIR="$TMP_DIR/data"
mkdir -p "$SYSKIT_DATA_DIR"

echo "==> 4/4 执行核心 help/smoke 命令"
go run ./cmd/syskit --help >/dev/null 2>&1
set +e
go run ./cmd/syskit doctor all --fail-on never --format json >/dev/null 2>&1
doctor_exit=$?
set -e
if [ "$doctor_exit" -ne 0 ] && [ "$doctor_exit" -ne 1 ]; then
    echo "doctor all smoke 失败，退出码: $doctor_exit"
    exit "$doctor_exit"
fi
go run ./cmd/syskit disk --format json >/dev/null 2>&1
go run ./cmd/syskit disk scan . --limit 3 --format json >/dev/null 2>&1
go run ./cmd/syskit policy init --type config --output "$TMP_DIR/config.yaml" >/dev/null 2>&1
go run ./cmd/syskit policy validate "$TMP_DIR/config.yaml" --type config --format json >/dev/null 2>&1
go run ./cmd/syskit snapshot list --limit 1 --format json >/dev/null 2>&1

echo "P0 验证完成。"
