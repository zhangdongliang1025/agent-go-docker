#!/usr/bin/env python3
"""
Playwright 无头浏览器示例脚本

此脚本演示如何在容器中使用 Playwright 进行无头浏览器自动化。
镜像已内置 Chromium 浏览器，无需额外安装。

使用方法:
    python3 examples/playwright_example.py

或者直接在容器中运行:
    docker run -it --rm \
      --network=host \
      -v "$PWD:/data" \
      -w "/data" \
      ghcr.io/mark0725/agent-go-docker:latest \
      python3 examples/playwright_example.py
"""

from playwright.sync_api import sync_playwright


def main():
    with sync_playwright() as p:
        # 启动无头浏览器
        browser = p.chromium.launch(headless=True)
        page = browser.new_page()

        # 访问网页
        page.goto("https://example.com")

        # 获取页面标题
        title = page.title()
        print(f"页面标题: {title}")

        # 获取页面内容
        content = page.content()
        print(f"页面内容长度: {len(content)} 字符")

        # 截图保存
        page.screenshot(path="/tmp/example.png")
        print("截图已保存到 /tmp/example.png")

        # 关闭浏览器
        browser.close()

        print("✅ Playwright 测试完成!")


if __name__ == "__main__":
    main()
