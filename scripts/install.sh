#!/bin/bash
#
# oss-pages CLI macOS 安装脚本
# 使用方法: curl -sSL https://raw.githubusercontent.com/oss-pages/oss-pages/main/scripts/install.sh | bash
#

set -e

REPO="oss-pages/oss-pages"
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="oss-cli"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# 检测 macOS
detect_os() {
    if [[ "$(uname)" != "Darwin" ]]; then
        log_error "此脚本仅支持 macOS"
        exit 1
    fi
    log_info "检测到 macOS 系统"
}

# 检查依赖
check_deps() {
    if ! command -v go &> /dev/null; then
        log_error "未找到 Go，请先安装: https://go.dev/dl/"
        exit 1
    fi
    log_info "Go 已安装: $(go version)"
}

# 获取最新版本
get_latest_version() {
    # 尝试从 GitHub 获取最新 release
    local version
    version=$(curl -sSL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/' || echo "")

    if [[ -z "$version" ]]; then
        # 如果无法获取，使用默认值
        version="latest"
        log_warn "无法获取最新版本，将使用源码编译"
    else
        log_info "最新版本: $version"
    fi

    echo "$version"
}

# 编译安装
build_install() {
    local install_path="$INSTALL_DIR/$BINARY_NAME"

    log_info "正在编译 CLI..."

    # 切换到临时目录
    local temp_dir
    temp_dir=$(mktemp -d)
    cd "$temp_dir"

    # 克隆或使用本地源码（优先使用脚本所在目录的相对路径）
    local script_dir="$(cd "$(dirname "$0")" && pwd)"
    local repo_root="$(dirname "$script_dir")"

    if [[ -d "$repo_root" && -f "$repo_root/src/cmd/cli/main.go" ]]; then
        log_info "使用本地源码"
        cd "$repo_root"
    else
        log_info "克隆源码..."
        git clone "https://github.com/${REPO}.git" "$temp_dir/repo" 2>/dev/null || {
            log_error "克隆失败"
            exit 1
        }
        cd "$temp_dir/repo"
    fi

    # 编译
    cd src
    go build -o "$temp_dir/$BINARY_NAME" ./cmd/cli

    # 安装
    if [[ -w "$INSTALL_DIR" ]]; then
        cp "$temp_dir/$BINARY_NAME" "$install_path"
        log_info "已安装到: $install_path"
    else
        log_warn "$INSTALL_DIR 需要 root 权限"
        sudo cp "$temp_dir/$BINARY_NAME" "$install_path"
        log_info "已安装到: $install_path (sudo)"
    fi

    chmod +x "$install_path"

    # 清理
    rm -rf "$temp_dir"

    log_info "安装完成!"
}

# 验证安装
verify_install() {
    local install_path="$INSTALL_DIR/$BINARY_NAME"

    if [[ -x "$install_path" ]]; then
        log_info "验证安装:"
        "$install_path" --version 2>/dev/null || "$install_path" --help
    else
        log_error "安装验证失败"
        exit 1
    fi
}

# 添加到 PATH 提示
hint_path() {
    local shell_config=""
    local profile=""

    # 检测 shell
    if [[ -n "$BASH_VERSION" ]]; then
        shell_config="$HOME/.bashrc"
        profile="$HOME/.bash_profile"
    elif [[ -n "$ZSH_VERSION" ]]; then
        shell_config="$HOME/.zshrc"
    fi

    echo ""
    log_info "=========================================="
    log_info "安装完成!"
    log_info ""
    log_info "安装路径: $INSTALL_DIR/$BINARY_NAME"
    echo ""

    # 检查是否在 PATH 中
    if ! command -v "$BINARY_NAME" &> /dev/null; then
        log_warn "$BINARY_NAME 不在当前 PATH 中"
        echo "请将以下内容添加到您的 shell 配置文件 ($shell_config):"
        echo ""
        echo "  # oss-pages CLI"
        echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
        echo ""
        echo "然后运行: source $shell_config"
    else
        log_info "$BINARY_NAME 已在 PATH 中"
    fi
    log_info "=========================================="
}

# 主流程
main() {
    echo "=========================================="
    echo "  oss-pages CLI 安装脚本 (macOS)"
    echo "=========================================="
    echo ""

    detect_os
    check_deps
    build_install
    verify_install
    hint_path
}

main "$@"
