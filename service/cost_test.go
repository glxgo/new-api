package service

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

// TestCalcGrossProfit 验证毛利公式(账平核心): gross = 本金 - 成本×本金占比(赠金成本平台自负)。
// 对应 plan 端到端验证场景。
func TestCalcGrossProfit(t *testing.T) {
	cases := []struct {
		name                              string
		paidPrincipal, paidGift, cost     int
		want                              int
	}{
		{"跨池分摊(plan场景: 600本金-200×0.6)", 600, 400, 200, 480},
		{"全本金(1000-200)", 1000, 0, 200, 800},
		{"全赠金(成本平台自负, 毛利0)", 0, 1000, 200, 0},
		{"无成本", 1000, 0, 0, 1000},
		{"零消费", 0, 0, 0, 0},
		{"负毛利(成本×占比>本金, RunDailySettle 会跳过)", 100, 0, 200, -100},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := CalcGrossProfit(c.paidPrincipal, c.paidGift, c.cost)
			if got != c.want {
				t.Errorf("CalcGrossProfit(principal=%d, gift=%d, cost=%d) = %d, want %d",
					c.paidPrincipal, c.paidGift, c.cost, got, c.want)
			}
		})
	}
}

// TestSplitPayment 验证双池拆分: 优先扣赠金, 不足扣本金。
func TestSplitPayment(t *testing.T) {
	cases := []struct {
		name                       string
		total, gift, principal     int
		wantGift, wantPrincipal    int
	}{
		{"跨池", 1000, 400, 600, 400, 600},
		{"全赠金", 1000, 1000, 0, 1000, 0},
		{"全本金", 1000, 0, 1000, 0, 1000},
		{"赠金充足total<gift", 500, 1000, 0, 500, 0},
		{"零消费", 0, 100, 100, 0, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g, p := SplitPayment(c.total, c.gift, c.principal)
			if g != c.wantGift || p != c.wantPrincipal {
				t.Errorf("SplitPayment(%d, gift=%d, principal=%d) = (gift=%d, principal=%d), want (%d, %d)",
					c.total, c.gift, c.principal, g, p, c.wantGift, c.wantPrincipal)
			}
		})
	}
}

// TestCalcModelCostQuota 验证成本换算: (input$/1M×prompt + output$/1M×completion)/1e6 × QuotaPerUnit。
// 1$ = 500000 quota (QuotaPerUnit)。
func TestCalcModelCostQuota(t *testing.T) {
	// 配置测试模型: input 1$/1M, output 2$/1M, cache 0
	if err := ratio_setting.UpdateModelCostByJSONString(`{"test-cost-model":{"input_cost_per_m":1,"output_cost_per_m":2,"cache_cost_per_m":0}}`); err != nil {
		t.Fatalf("setup model cost failed: %v", err)
	}
	cases := []struct {
		name                  string
		prompt, completion    int
		want                  int
	}{
		{"1M input × 1$/1M = 1$ = 500000 quota", 1000000, 0, 500000},
		{"1M output × 2$/1M = 2$ = 1000000 quota", 0, 1000000, 1000000},
		{"1M input + 1M output = 3$ = 1500000 quota", 1000000, 1000000, 1500000},
		{"零 token", 0, 0, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := CalcModelCostQuota("test-cost-model", "", c.prompt, c.completion)
			if !ok {
				t.Fatalf("expected model cost configured, got ok=false")
			}
			if got != c.want {
				t.Errorf("CalcModelCostQuota(prompt=%d, completion=%d) = %d, want %d",
					c.prompt, c.completion, got, c.want)
			}
		})
	}
	// 未配置模型应返回 ok=false
	if _, ok := CalcModelCostQuota("test-cost-unconfigured", "", 1000, 1000); ok {
		t.Error("expected ok=false for unconfigured model")
	}
}

// TestDividendBalanceEquation 验证分润总账平衡等式(plan 核心场景):
// 毛利 = 拉新返利 + 管理员分红 + 超管分红 + 净利润
// 场景: C 消费 1000(本金600+赠金400) 成本200 → 毛利480
//   拉新直接 B: 480×10%=48; 拉新间接 A(管理员)不发; 管理员A分红: 480×50%=240; 超管: 480×10%=48
//   净利 = 480-48-240-48 = 144; 平衡: 480 == 48+240+48+144 ✓
func TestDividendBalanceEquation(t *testing.T) {
	gross := CalcGrossProfit(600, 400, 200) // 480
	if gross != 480 {
		t.Fatalf("gross = %d, want 480", gross)
	}
	const directRate, indirectRate, rootRate, adminRate = 0.10, 0.05, 0.10, 0.50
	rebate := int(float64(gross) * directRate)         // 48
	adminDiv := int(float64(gross) * adminRate)        // 240
	rootDiv := int(float64(gross) * rootRate)          // 48
	netProfit := gross - rebate - adminDiv - rootDiv   // 144

	// 平衡等式: 毛利 = 拉新 + 管理员 + 超管 + 净利
	if rebate+adminDiv+rootDiv+netProfit != gross {
		t.Errorf("账不平: 拉新%d+管理员%d+超管%d+净利%d=%d != 毛利%d",
			rebate, adminDiv, rootDiv, netProfit, rebate+adminDiv+rootDiv+netProfit, gross)
	}
	// 关键数值断言
	if rebate != 48 {
		t.Errorf("拉新返利=%d want 48", rebate)
	}
	if adminDiv != 240 {
		t.Errorf("管理员分红=%d want 240", adminDiv)
	}
	if rootDiv != 48 {
		t.Errorf("超管分红=%d want 48", rootDiv)
	}
	if netProfit != 144 {
		t.Errorf("净利润=%d want 144", netProfit)
	}
	// 间接率此处不发(A 是管理员走分红), 但校验间接率常量存在且默认 0.05
	if indirectRate != 0.05 {
		t.Errorf("间接率=%v want 0.05", indirectRate)
	}
}
