#!/usr/bin/env python3
"""
Playwright 无头浏览器高级示例

此脚本演示更多 Playwright 功能：
- 表单填写
- 点击操作
- 等待元素
- 多页面处理

使用方法:
    python3 examples/playwright_advanced.py
"""

from playwright.sync_api import sync_playwright


def search_on_baidu(page, query):
    """在百度上搜索"""
    # 访问百度
    page.goto("https://www.baidu.com")

    # 填写搜索框
    page.fill("#kw", query)

    # 点击搜索按钮
    page.click("#su")

    # 等待搜索结果出现
    page.wait_for_selector("#content_left", timeout=10000)

    # 获取搜索结果标题
    results = page.query_selector_all("#content_left .result h3")
    titles = [result.inner_text() for result in results[:5]]

    return titles


def main():
    with sync_playwright() as p:
        # 启动浏览器
        browser = p.chromium.launch(headless=True)
        page = browser.new_page()

        # 设置视口大小
        page.set_viewport_size({"width": 1920, "height": 1080})

        # 搜索示例
        query = "Playwright Python"
        print(f"🔍 正在搜索: {query}")

        titles = search_on_baidu(page, query)

        print(f"\n搜索结果 ({len(titles)} 条):")
        for i, title in enumerate(titles, 1):
            print(f"  {i}. {title}")

        # 截图保存
        page.screenshot(path="/tmp/baidu_search.png")
        print("\n📸 截图已保存到 /tmp/baidu_search.png")

        # 关闭浏览器
        browser.close()

        print("\n✅ 高级 Playwright 测试完成!")


if __name__ == "__main__":
    main()
