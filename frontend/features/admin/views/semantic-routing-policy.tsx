import type { Model, SemanticRoutingPolicy } from "../core/types";
import { tx } from "../i18n/runtime";

export const semanticRoutingMetadataKey = "tokenhub_semantic_routing";

export function readSemanticRoutingPolicy(model: Model): SemanticRoutingPolicy {
  const fallback: SemanticRoutingPolicy = { mode: "off", min_confidence: 0.65 };
  try {
    const parsed = JSON.parse(model.metadata?.[semanticRoutingMetadataKey] ?? "null");
    if (parsed && ["off", "shadow", "enforce"].includes(parsed.mode) && typeof parsed.min_confidence === "number" && Number.isFinite(parsed.min_confidence) && parsed.min_confidence >= 0 && parsed.min_confidence <= 1) return parsed;
  } catch { /* Invalid legacy metadata leaves semantic routing disabled. */ }
  return fallback;
}

export function SemanticRoutingFields({ value, disabled, onChange }: {
  value: SemanticRoutingPolicy;
  disabled: boolean;
  onChange: (value: SemanticRoutingPolicy) => void;
}) {
  return (
    <fieldset className="semantic-routing-fields" disabled={disabled}>
      <legend>{tx("Jev 智能路由")}</legend>
      <div className="form-grid">
        <label className="field">
          <span>{tx("智能路由模式")}</span>
          <select value={value.mode} onChange={(event) => {
            const mode = event.target.value as SemanticRoutingPolicy["mode"];
            const validConfidence = Number.isFinite(value.min_confidence) && value.min_confidence >= 0 && value.min_confidence <= 1;
            onChange({ ...value, mode, min_confidence: mode === "off" && !validConfidence ? 0.65 : value.min_confidence });
          }}>
            <option value="off">{tx("关闭智能路由")}</option>
            <option value="shadow">{tx("观察：记录建议，保留原路由")}</option>
            <option value="enforce">{tx("启用：按建议选择线路")}</option>
          </select>
        </label>
        <label className="field">
          <span>{tx("最低置信度")}</span>
          <input type="number" min="0" max="1" step="0.01" value={Number.isFinite(value.min_confidence) ? value.min_confidence : ""} disabled={value.mode === "off"} onChange={(event) => onChange({ ...value, min_confidence: event.target.value === "" ? NaN : Number(event.target.value) })} />
        </label>
      </div>
      <p>{tx("仅在当前模型的同优先级候选中选择；失败或低置信度时沿用基础策略。置信度不代表任务成功率。")}</p>
      <p>{tx("仅适用于无工具、无会话绑定的纯文本 Chat Completions。服务端须开启功能并允许当前项目向 TypeSafe 发送用户文本；观察模式同样会发送文本。")}</p>
    </fieldset>
  );
}
