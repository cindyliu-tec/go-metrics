# go-metrics



## 组件介绍
这是一个基于gin框架的组件，提供一个请求处理中间件，所有请求都会由此经过。过程中会读取请求header、响应状态码、响应体code字段，以采集接口指标所需字段，并通过专用端口暴露出指标数据供prometheus采集。
目前采集以下4项指标：

1、记录普通请求的请求个数、耗时、业务code等
``mk_gin_uri_request_duration_sum{app="demo-project", code="-1", container="demo-project", endpoint="metrics", env="dev", httpcode="200", method="GET", namespace="dev-common", pod="demo-project-8d9bd5754-nd8tc", uri="/demo-api/healthz"}``

2、记录普通请求响应时间超过10s的慢请求个数
``mk_gin_slow_request_total{app="demo-project", code="0", container="demo-project", endpoint="metrics", env="dev", httpcode="200", method="GET", namespace="dev-common", pod="demo-project-554657784d-v9xff", uri="/demo-api/v1/testPathParam/:num"}``

3、记录非普通请求个数(websocket、文件上传等)
``mk_gin_long_request_total{app="demo-project", code="-1", container="demo-project", endpoint="metrics", env="dev", httpcode="200", method="POST", namespace="dev-common", pod="demo-project-8d9bd5754-nd8tc", uri="/demo-api/v1/testUploadFile"}``

4、记录当前使用的sdk版本
``mk_monitor_sdk_version{app="demo-project", container="demo-project", endpoint="metrics", env="dev", namespace="dev-common", pod="demo-project-554657784d-fc75p", version="0.0.1"}``

## 使用指南
https://makeblock.feishu.cn/docx/EzrldrvqeoyLsuxeTW1cLk6SnTf?from=from_copylink
