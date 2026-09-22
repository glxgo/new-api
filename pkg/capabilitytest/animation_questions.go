package capabilitytest

import "fmt"

// These combinations are explicit, not a Cartesian product of unrelated nouns.
// Structural tests do not replace per-template human visual calibration.
var animationTemplates = [...]struct {
	id, subject, object, action, relation string
}{
	{"pelican-bicycle-v1", "鹈鹕", "自行车", "骑自行车", "骑乘"},
	{"panda-bicycle-v1", "熊猫", "自行车", "骑自行车", "骑乘"},
	{"rabbit-scooter-v1", "兔子", "滑板车", "骑滑板车", "骑乘"},
	{"fox-skateboard-v1", "狐狸", "滑板", "滑滑板", "站立滑行"},
	{"robot-drums-v1", "机器人", "鼓", "打鼓", "敲击"},
	{"robot-juggling-v1", "机器人", "球", "抛接球", "抛接"},
	{"cat-swing-v1", "猫", "秋千", "荡秋千", "乘坐"},
	{"penguin-rowing-v1", "企鹅", "小船", "划小船", "乘船划行"},
	{"astronaut-rope-v1", "宇航员", "跳绳", "跳绳", "持绳跳跃"},
	{"dog-bicycle-v1", "狗", "自行车", "骑自行车", "骑乘"},
	{"bear-scooter-v1", "熊", "滑板车", "骑滑板车", "骑乘"},
	{"squirrel-skateboard-v1", "松鼠", "滑板", "滑滑板", "站立滑行"},
}

// AnimationTemplateIDs returns a copy so calibration cannot mutate the pool.
func AnimationTemplateIDs() []string {
	ids := make([]string, len(animationTemplates))
	for i, template := range animationTemplates {
		ids[i] = template.id
	}
	return ids
}

func animationQuestion(seedHash [32]byte) Question {
	template := animationTemplates[int(seedHash[7])%len(animationTemplates)]
	return Question{
		Kind: "scene", Format: "html-svg-animation", TemplateID: template.id,
		Prompt: fmt.Sprintf("生成 html，内容是 svg 绘制%s%s 2D 动画，不进行测试，不使用 skill，不参考本地文件。", template.subject, template.action),
		Requirements: []string{
			"可辨识的" + template.subject + "主体",
			"可辨识的" + template.object,
			template.subject + "与" + template.object + "呈" + template.relation + "关系",
			"主体以二维图形呈现",
			"观察窗口内" + template.subject + "或" + template.object + "呈现运动",
			"跨帧动作支持" + template.action + "表现，而非仅背景变化",
		},
	}
}
