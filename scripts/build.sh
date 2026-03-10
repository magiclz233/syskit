#!/bin/bash
# Find Large Files - 跨平台编译脚本
# 用法: ./build.sh [target]
#
# 参数:
#   (无参数)          - 编译当前平台版本
#   all              - 编译所有平台版本
#   windows          - 编译所有 Windows 版本
#   windows-amd64    - 编译 Windows 64位版本
#   windows-386      - 编译 Windows 32位版本
#   windows-arm64    - 编译 Windows ARM64版本
#   linux            - 编译所有 Linux 版本
#   linux-amd64      - 编译 Linux 64位版本
#   linux-386        - 编译 Linux 32位版本
#   linux-arm64      - 编译 Linux ARM64版本
#   linux-arm        - 编译 Linux ARM32版本
#   darwin           - 编译所有 macOS 版本
#   darwin-amd64     - 编译 macOS Intel版本
#   darwin-arm64     - 编译 macOS Apple Silicon版本

set -e

# Get script directory and project root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

# 颜色定义
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 项目信息
APP_NAME="find-large-files"
BUILD_DIR="build"

# 创建 build 目录
mkdir -p "$BUILD_DIR"

# 编译函数
build() {
    local os=$1
    local arch=$2
    local ext=$3

    local output="${BUILD_DIR}/${APP_NAME}-${os}-${arch}${ext}"

    echo -e "${BLUE}正在编译 ${os}/${arch}...${NC}"
    GOOS=$os GOARCH=$arch go build -ldflags="-s -w" -o "$output"

    if [ $? -eq 0 ]; then
        local size=$(du -h "$output" | cut -f1)
        echo -e "${GREEN}✓ 编译完成: ${output} (${size})${NC}"
    else
        echo -e "${RED}✗ 编译失败: ${os}/${arch}${NC}"
        return 1
    fi
}

# 检测当前平台
detect_platform() {
    local os=$(uname -s | tr '[:upper:]' '[:lower:]')
    local arch=$(uname -m)

    case "$os" in
        linux*)
            os="linux"
            ;;
        darwin*)
            os="darwin"
            ;;
        mingw*|msys*|cygwin*)
            os="windows"
            ;;
    esac

    case "$arch" in
        x86_64|amd64)
            arch="amd64"
            ;;
        i386|i686)
            arch="386"
            ;;
        aarch64|arm64)
            arch="arm64"
            ;;
        armv7l)
            arch="arm"
            ;;
    esac

    echo "${os}-${arch}"
}

# 编译所有版本
build_all() {
    echo -e "${YELLOW}=== 编译所有平台版本 ===${NC}"
    echo ""

    # Windows
    build windows amd64 .exe
    build windows 386 .exe
    build windows arm64 .exe

    # Linux
    build linux amd64 ""
    build linux 386 ""
    build linux arm64 ""
    build linux arm ""

    # macOS
    build darwin amd64 ""
    build darwin arm64 ""

    echo ""
    echo -e "${GREEN}=== 所有版本编译完成 ===${NC}"
    echo ""
    ls -lh "$BUILD_DIR/"
}

# 编译 Windows 所有版本
build_windows() {
    echo -e "${YELLOW}=== 编译 Windows 版本 ===${NC}"
    echo ""
    build windows amd64 .exe
    build windows 386 .exe
    build windows arm64 .exe
    echo ""
    echo -e "${GREEN}=== Windows 版本编译完成 ===${NC}"
}

# 编译 Linux 所有版本
build_linux() {
    echo -e "${YELLOW}=== 编译 Linux 版本 ===${NC}"
    echo ""
    build linux amd64 ""
    build linux 386 ""
    build linux arm64 ""
    build linux arm ""
    echo ""
    echo -e "${GREEN}=== Linux 版本编译完成 ===${NC}"
}

# 编译 macOS 所有版本
build_darwin() {
    echo -e "${YELLOW}=== 编译 macOS 版本 ===${NC}"
    echo ""
    build darwin amd64 ""
    build darwin arm64 ""
    echo ""
    echo -e "${GREEN}=== macOS 版本编译完成 ===${NC}"
}

# 编译当前平台
build_current() {
    local platform=$(detect_platform)
    local os=$(echo $platform | cut -d'-' -f1)
    local arch=$(echo $platform | cut -d'-' -f2)
    local ext=""

    if [ "$os" = "windows" ]; then
        ext=".exe"
    fi

    echo -e "${YELLOW}=== 编译当前平台版本 (${os}/${arch}) ===${NC}"
    echo ""
    build $os $arch $ext
    echo ""
    echo -e "${GREEN}=== 编译完成 ===${NC}"
}

# 显示帮助
show_help() {
    echo "Find Large Files - 跨平台编译脚本"
    echo ""
    echo "用法: ./build.sh [target]"
    echo ""
    echo "参数:"
    echo "  (无参数)          - 编译当前平台版本"
    echo "  all              - 编译所有平台版本"
    echo "  windows          - 编译所有 Windows 版本"
    echo "  windows-amd64    - 编译 Windows 64位版本"
    echo "  windows-386      - 编译 Windows 32位版本"
    echo "  windows-arm64    - 编译 Windows ARM64版本"
    echo "  linux            - 编译所有 Linux 版本"
    echo "  linux-amd64      - 编译 Linux 64位版本"
    echo "  linux-386        - 编译 Linux 32位版本"
    echo "  linux-arm64      - 编译 Linux ARM64版本"
    echo "  linux-arm        - 编译 Linux ARM32版本"
    echo "  darwin           - 编译所有 macOS 版本"
    echo "  darwin-amd64     - 编译 macOS Intel版本"
    echo "  darwin-arm64     - 编译 macOS Apple Silicon版本"
    echo "  help             - 显示此帮助信息"
    echo ""
    echo "示例:"
    echo "  ./build.sh                  # 编译当前平台"
    echo "  ./build.sh all              # 编译所有平台"
    echo "  ./build.sh windows-amd64    # 只编译 Windows 64位"
    echo "  ./build.sh darwin           # 编译所有 macOS 版本"
}

# 主逻辑
main() {
    local target=${1:-current}

    case "$target" in
        all)
            build_all
            ;;
        windows)
            build_windows
            ;;
        windows-amd64)
            build windows amd64 .exe
            ;;
        windows-386)
            build windows 386 .exe
            ;;
        windows-arm64)
            build windows arm64 .exe
            ;;
        linux)
            build_linux
            ;;
        linux-amd64)
            build linux amd64 ""
            ;;
        linux-386)
            build linux 386 ""
            ;;
        linux-arm64)
            build linux arm64 ""
            ;;
        linux-arm)
            build linux arm ""
            ;;
        darwin)
            build_darwin
            ;;
        darwin-amd64)
            build darwin amd64 ""
            ;;
        darwin-arm64)
            build darwin arm64 ""
            ;;
        current)
            build_current
            ;;
        help|--help|-h)
            show_help
            ;;
        *)
            echo "错误: 未知的目标 '$target'"
            echo ""
            show_help
            exit 1
            ;;
    esac
}

# 执行主函数
main "$@"
