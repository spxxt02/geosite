# Geosite Data Generator

[![GitHub Workflow Status](https://img.shields.io/github/actions/workflow/status/lhear/geosite/.github/workflows/build.yml?branch=main)](https://github.com/lhear/geosite/actions/workflows/build.yml)
[![GitHub license](https://img.shields.io/github/license/lhear/geosite)](LICENSE)

**目的**: 从自定义 URL 列表生成 V2Ray / Xray 兼容的 `geosite.dat` 文件。

**自动化**: 使用 GitHub Actions 自动构建并发布最新的 `geosite.dat`。

---

## 快速使用 (获取 `geosite.dat`)

1.  下载 `geosite.dat` 文件 [https://raw.githubusercontent.com/lhear/geosite/release/geosite.dat](https://raw.githubusercontent.com/lhear/geosite/release/geosite.dat)。
2.  将其放入你的 V2Ray / Xray 配置目录。

---

## 本地构建 (开发者)

### 1. 环境

*   Go (1.24+)
*   Git

### 2. 克隆与配置

```bash
git clone https://github.com/lhear/geosite.git
cd geosite
# 编辑 urls.txt 文件
