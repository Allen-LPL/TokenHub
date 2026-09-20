# Jev 智能路由

TokenHub 可以调用 TypeSafe 的 Jev 模型，为符合条件的 Chat Completions 请求选择 Provider 和上游模型。Jev 使用独立的决策客户端，无需注册为 TokenHub 的 Provider。最终执行任务的模型必须已有有效路由，且 Provider 适配器支持请求协议。

## 工作机制

1. TokenHub 完成认证、额度准入、隐私处理和安全检查。命中响应缓存时直接返回，不调用 Jev。
2. 项目与 API Key 权限、作用域策略、健康检查、协议兼容性、优先级和基础算法共同生成候选顺序。
3. 模型与项目均已开启功能时，Jev 接收合格候选的能力信息及用户消息文本，通过 TypeSafe 的 `Choice` 原语返回一个候选或 `no_preference`。
4. 「启用」模式下，有效选择达到置信度阈值后，对应的 Provider 与模型会被移到首位，但只限于当前最高路由优先级和资源优先级。该 Provider 与模型内部的资源顺序、其他候选的相对顺序均保留。「观察」模式只记录建议。
5. 原有调用与故障转移流程使用调整后的顺序。每个请求最多调用一次 Jev，上游失败后不会重新判断，也不会重试 Jev。

原有六种基础策略继续可用。严格主备策略的最高优先级通常只有一个候选，此时不会调用 Jev。路由模拟器仍然解释基础策略，不调用 Jev。

## 开启与回滚

功能默认关闭。服务端配置示例：

```dotenv
TOKENHUB_SEMANTIC_ROUTING_ENABLED=true
TOKENHUB_SEMANTIC_ROUTING_PROJECTS=prj_example
TOKENHUB_TYPESAFE_API_KEY=<server-side-secret>
TOKENHUB_TYPESAFE_MODEL=jev-1.13.0
TOKENHUB_SEMANTIC_ROUTING_TIMEOUT_MS=1000
```

项目名单使用逗号分隔的精确项目 ID。空名单不允许外发；开启功能但缺少密钥或项目名单时，启动检查会报错。应使用明确的评估模型版本，响应版本不匹配时回退。超时包含目录查询和评估，格式错误或超过 10000 ms 的值会被拒绝。每个进程最多同时执行 8 次 Jev 调用，超出的请求立即保留原路由。

在「路由策略 → 模型路由策略 → Jev 智能路由」中选择：

- 「关闭」：不评估，也不外发文本。
- 「观察」（`shadow`）：评估并记录审计，保持基础顺序。
- 「启用」（`enforce`）：建议有效且达到阈值时调整顺序。

初始阈值为 `0.65`，仅作为评估起点，不代表已验证的质量保证。Jev 置信度描述选项分布，不是模型完成任务的成功率。开启执行前，应使用带有期望结果的业务样本检查观察模式的建议。

模型级回滚可直接选择「关闭」。全局回滚可设置 `TOKENHUB_SEMANTIC_ROUTING_ENABLED=false` 并重启。两者均不会删除路由或修改基础策略。

## 候选信息与统一模型别名

候选按 Provider ID 和上游模型精确分组，同一资源池的多个账户不会成为重复选项。Jev 只能选择当前统一模型已经通过准入的路由，不能添加 Provider、任意选择模型或绕过权限。

跨生成模型选择时，需要显式创建 `auto-chat` 等统一模型别名，并将其路由映射到批准使用的上游模型。现有模型名称不会自动增加线路。统一模型原有的定价和计费规则继续生效，多模型别名不会按每次选中的上游模型自动重新定价。

发送给 Jev 的信息包括 Provider 类型、上游模型名、能力、输入模态、支持参数，以及可选的 `ProviderModel.metadata.routing_description`。描述应来自可验证的任务适用性，例如已评估的代码或翻译能力。凭据、地址、项目名、其他元数据、价格和运行健康记录不发送。提示词要求 Jev 不编造品牌排名、基准成绩或价格；缺乏能力证据和业务评估时，本功能不能证明某个模型最佳。

目录中已声明的文本模态、上下文上限和参数支持会限制智能选择。缺少目录条目的模型不能被提升；缺少能力描述也不代表更优。单个候选信息超过 2048 字节时不发送；不同 Provider 与模型组合超过 32 个时跳过本次评估。

## 请求范围与数据处理

首版支持纯文本 `/v1/chat/completions`，包括流式请求。工具、结构化输出约束、多模态消息、推理续接、会话标识、缓存亲和性、粘性路由，以及未知请求或消息字段均跳过。Responses、Anthropic Messages、向量、图片 API 和路由模拟保持原行为。

「观察」和「启用」都会把经过处理的用户消息文本及有限候选信息发送至 `https://api.typesafe.ai/v1/systemone`。系统消息、开发者消息、助手消息、请求头、API Key 和 Provider 凭据均不发送。用户文本含分隔符不得超过 8192 字节；超限时保留原路由，不截断文本。项目名单应仅包含允许使用该外部服务的业务。用户文本和能力描述均作为数据处理，本地校验始终限制选择范围。

超时、取消、并发已满、限流、过载、响应格式错误或超限、未知选项、低置信度，以及 `no_preference` 都会保留基础顺序。客户端不跟随重定向，TypeSafe 地址固定，认证保留在服务端。

审计事件使用 `routing.semantic` 动作及网关请求 ID，记录模式、结果、提示词版本、评估模型版本、置信度、选中的本地路由 ID、评估 Token 数和决策耗时，不包含提示词、响应正文或凭据。评估 Token 不会加入生成模型的用量或客户账单，TypeSafe 费用需单独预算。全局关闭或项目未获允许时不生成此类审计事件。

## API 兼容性

现有 `PATCH /api/admin/model-routing-policies/{model}` 在原有 `strategy` 和完整 `routes` 列表之外，接受可选字段：

```json
{
  "semantic_routing": {
    "mode": "shadow",
    "min_confidence": 0.65
  }
}
```

示例只展示新增字段，实际 PATCH 仍需提交基础策略和每条路由。模式与明确的有限置信度阈值均必填，阈值范围为 0 到 1。设置或路由无效时整个更新回滚。旧客户端省略 `semantic_routing` 时保留已保存的设置。策略存储于 `Model.metadata.tokenhub_semantic_routing`，无需新增数据库表或迁移。

## 验证与评估

自动化测试使用合成请求、进程内 HTTP 服务和模拟生成 Provider，覆盖 TypeSafe 协议、候选约束、响应校验、模式、适用范围、阈值边界、资源池、权限、取消、超时、故障转移、原子存储，以及界面的保存与重新加载。测试不代表真实业务准确率或生产延迟。

建议先使用「观察」模式，检查审计结果与新增耗时，并在获准的代表性样本上评估任务成功率、生成费用和评估费用，再按项目与模型开启执行。

协议参考：[TypeSafe API](https://docs.typesafe.ai/api)、[Choice](https://docs.typesafe.ai/primitives/choice)、[Confidence](https://docs.typesafe.ai/confidence)。

可选的真实 API 协议冒烟测试为 `TestJevRoutingClientLive`。仅在测试进程环境中设置 `TOKENHUB_LIVE_TYPESAFE_API_KEY`，然后在 `backend/` 运行 `go test ./internal/server -run ^TestJevRoutingClientLive$ -count=1 -v`。测试只向 TypeSafe 发送合成翻译请求，不调用生成 Provider，默认跳过。

已有模型的智能路由元数据由路由策略接口管理。普通模型编辑、过期的元数据请求和启动时目录刷新均保留当前设置。模型与策略写入统一先锁定模型、再处理路由，防止并发编辑恢复旧的外发设置。
