import type { ModelRoute, ModelRoutePolicy, Provider } from "../../features/admin/core/types";
import { test, expect, capture } from "./harness";
import { model, shellResponses } from "./fixtures/shell";

for (const state of ["save", "failure", "mobile"] as const) {
  test(`semantic-routing ${state}`, async ({ page, api }, testInfo) => {
    if (state === "mobile") await page.setViewportSize({ width: 390, height: 844 });
    const routes: ModelRoute[] = [0, 1].map(index => ({ id: `route_ui_${index}`, model_name: model.name, provider_id: `provider_ui_${index}`, provider_model: `upstream-${index}`, priority: 1, weight: 100, quality_score: 50, cost_score: 50, status: "active", strategy: "quality" }));
    const providers: Provider[] = [0, 1].map(index => ({ id: `provider_ui_${index}`, name: `UI Provider ${index}`, type: "mock", priority: 1, healthy: true, status: "active" }));
    api.respond("GET", "/api/admin/providers", { data: providers });
    api.respond("GET", "/api/admin/routing-rules", { data: routes });
    for (const path of ["provider-resources", "plugins", "provider-adapters", "plugin-ui-manifest", "plugin-actions", "plugin-background-jobs"]) api.respond("GET", `/api/admin/${path}`, { data: [] });
    const saved: ModelRoutePolicy[] = [];
    api.define("PATCH", `/api/admin/model-routing-policies/${model.name}`, input => {
      const policy = input.body as ModelRoutePolicy;
      expect(policy.strategy).toBe("quality");
      expect(policy.routes).toEqual(routes.map(route => ({ route_id: route.id, weight: 100, quality_score: 50, cost_score: 50 })));
      expect(policy.semantic_routing?.min_confidence).toBe(0.8);
      saved.push(policy);
      if (state === "failure") return { status: 500, json: { error: { message: "Synthetic save failure" } } };
      const overview = shellResponses().get("GET /api/admin/overview") as Record<string, unknown>;
      api.replaceResponse("GET", "/api/admin/overview", { ...overview, models: [{ ...model, metadata: { tokenhub_semantic_routing: JSON.stringify(policy.semantic_routing) } }] });
      return { json: { strategy: policy.strategy, data: routes, semantic_routing: policy.semantic_routing } };
    });
    await page.goto("/routes");
    const panel = page.getByRole("group", { name: "Jev 智能路由" });
    await expect(panel.getByLabel("智能路由模式")).toHaveValue("off");
    await panel.getByLabel("智能路由模式").selectOption("shadow");
    await panel.getByLabel("最低置信度").fill("1.5");
    await expect(page.getByRole("button", { name: "应用策略" })).toBeDisabled();
    await panel.getByLabel("最低置信度").fill("0.8");
    await page.getByRole("button", { name: "应用策略" }).click();
    if (state === "failure") {
      await expect(page.getByText("Synthetic save failure")).toBeVisible();
      await expect(panel.getByLabel("智能路由模式")).toHaveValue("shadow");
      await expect(page.getByRole("button", { name: "应用策略" })).toBeEnabled();
    } else {
      await expect(page.getByRole("button", { name: "应用策略" })).toBeDisabled();
      await expect(panel.getByLabel("智能路由模式")).toHaveValue("shadow");
      await page.reload();
      await expect(panel.getByLabel("智能路由模式")).toHaveValue("shadow");
      if (state === "save") {
        for (const mode of ["enforce", "off"]) {
          await panel.getByLabel("智能路由模式").selectOption(mode);
          await page.getByRole("button", { name: "应用策略" }).click();
          await expect(page.getByRole("button", { name: "应用策略" })).toBeDisabled();
          await expect(panel.getByLabel("智能路由模式")).toHaveValue(mode);
        }
        expect(saved.map(policy => policy.semantic_routing?.mode)).toEqual(["shadow", "enforce", "off"]);
      }
    }
    await capture(page, testInfo, panel, `semantic-routing-${state}`, "Jev 智能路由设置");
  });
}
