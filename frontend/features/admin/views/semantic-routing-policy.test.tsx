import { render, screen, fireEvent } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { emptyData } from "../domain/catalog";
import type { Model, ModelRoute } from "../core/types";
import { ModelRoutingPolicyEditor, modelRoutePolicyPayload } from "./model-routing-policy";
import { readSemanticRoutingPolicy } from "./semantic-routing-policy";

const model: Model = { id: "m", name: "test-model", family: "test", modality: "chat", status: "active" };
const routes: ModelRoute[] = [{ id: "r", model_name: "test-model", provider_id: "p", provider_model: "upstream", priority: 1, weight: 100, quality_score: 50, cost_score: 50, status: "active", strategy: "quality" }];
function renderEditor(current = model) {
  const save = vi.fn();
  render(<ModelRoutingPolicyEditor model={current} routes={routes} data={emptyData()} loading={false} draggedRouteID="" onDragStart={vi.fn()} onDragEnd={vi.fn()} onDrop={vi.fn()} onEdit={vi.fn()} onDelete={vi.fn()} onSave={save} />);
  return save;
}

describe("Semantic routing policy", () => {
  it("saves observation mode with the unchanged base policy", () => {
    const save = renderEditor();
    expect(screen.getByRole("button", { name: "应用策略" })).toBeDisabled();
    fireEvent.change(screen.getByLabelText("智能路由模式"), { target: { value: "shadow" } });
    fireEvent.change(screen.getByLabelText("最低置信度"), { target: { value: "0.8" } });
    fireEvent.click(screen.getByRole("button", { name: "应用策略" }));
    expect(save).toHaveBeenCalledWith(model, { ...modelRoutePolicyPayload("quality", routes), semantic_routing: { mode: "shadow", min_confidence: 0.8 } });
  });
  it("loads persisted policy and rejects empty or out of range confidence", () => {
    renderEditor({ ...model, metadata: { tokenhub_semantic_routing: JSON.stringify({ mode: "enforce", min_confidence: 0.7 }) } });
    expect(screen.getByLabelText("智能路由模式")).toHaveValue("enforce");
    expect(screen.getByLabelText("最低置信度")).toHaveValue(0.7);
    for (const invalid of ["", "1.5", "-0.1"]) {
      fireEvent.change(screen.getByLabelText("最低置信度"), { target: { value: invalid } });
      expect(screen.getByRole("button", { name: "应用策略" })).toBeDisabled();
    }
    fireEvent.change(screen.getByLabelText("最低置信度"), { target: { value: "0" } });
    expect(screen.getByRole("button", { name: "应用策略" })).toBeEnabled();
  });
  it("disables the overlay while preserving legacy reorder payloads", () => {
    const save = renderEditor({ ...model, metadata: { tokenhub_semantic_routing: '{"mode":"shadow","min_confidence":0.65}' } });
    fireEvent.change(screen.getByLabelText("智能路由模式"), { target: { value: "off" } });
    fireEvent.click(screen.getByRole("button", { name: "应用策略" }));
    expect(save.mock.calls[0][1].semantic_routing.mode).toBe("off");
    expect(modelRoutePolicyPayload("priority_only", routes)).not.toHaveProperty("semantic_routing");
  });
  it("can disable routing after an invalid confidence edit", () => {
    const current = { ...model, metadata: { tokenhub_semantic_routing: '{"mode":"enforce","min_confidence":0.8}' } };
    const save = renderEditor(current);
    fireEvent.change(screen.getByLabelText("最低置信度"), { target: { value: "" } });
    fireEvent.change(screen.getByLabelText("智能路由模式"), { target: { value: "off" } });
    fireEvent.click(screen.getByRole("button", { name: "应用策略" }));
    expect(save).toHaveBeenCalledWith(current, expect.objectContaining({ semantic_routing: { mode: "off", min_confidence: 0.65 } }));
  });
  it.each(["invalid", "null", '{"mode":"unknown","min_confidence":0.5}', '{"mode":"enforce","min_confidence":2}'])("defaults malformed metadata to off: %s", (raw) => {
    expect(readSemanticRoutingPolicy({ ...model, metadata: { tokenhub_semantic_routing: raw } })).toEqual({ mode: "off", min_confidence: 0.65 });
  });
});
