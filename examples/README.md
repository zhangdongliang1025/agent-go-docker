# 示例脚本

此目录包含各种示例脚本，演示如何在容器中使用不同功能。

## Playwright 无头浏览器示例

### 基础示例

```bash
# 在容器中运行基础示例
docker run -it --rm \
  --network=host \
  -v "$PWD:/data" \
  -w "/data" \
  ghcr.io/mark0725/agent-go-docker:latest \
  python3 examples/playwright_example.py
```

### 高级示例（表单填写、点击操作等）

```bash
# 在容器中运行高级示例
docker run -it --rm \
  --network=host \
  -v "$PWD:/data" \
  -w "/data" \
  ghcr.io/mark0725/agent-go-docker:latest \
  python3 examples/playwright_advanced.py
```

## 自定义脚本

你可以创建自己的 Playwright 脚本，镜像已内置所有依赖：

```python
from playwright.sync_api import sync_playwright

with sync_playwright() as p:
    browser = p.chromium.launch(headless=True)
    page = browser.new_page()
    page.goto("https://example.com")
    print(page.title())
    browser.close()
```

## 注意事项

- 所有示例都使用 `headless=True` 模式，无需图形界面
- 截图会保存到 `/tmp/` 目录，你可以挂载卷来持久化
- 网络访问需要 `--network=host` 或适当的网络配置
