package capabilitytest

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQuestionsAnimationPoolFrozenAndCovered(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		seed := fmt.Sprintf("animation-pool-%d", i)
		questions := Generate(seed)
		require.Equal(t, questions, Generate(seed))
		require.Len(t, questions, 3, "random selection must not increase calls per target")
		q := questions[2]
		seen[q.TemplateID] = true
		require.Equal(t, questionHash(q), q.Hash)
		require.Contains(t, q.Prompt, "生成 html，内容是 svg 绘制")
		require.Contains(t, q.Prompt, "2D 动画，不进行测试，不使用 skill，不参考本地文件。")
		require.Len(t, q.Requirements, 6)
		if q.TemplateID == "pelican-bicycle-v1" {
			require.Equal(t, PelicanPrompt, q.Prompt)
		}
	}
	require.Len(t, seen, len(animationTemplates))
	for _, template := range animationTemplates {
		require.True(t, seen[template.id])
	}
}

func TestQuestionsAnimationRubricsMatchEachPrompt(t *testing.T) {
	for i, template := range animationTemplates {
		q := animationQuestion([32]byte{7: byte(i)})
		require.Contains(t, q.Prompt, template.subject+template.action)
		require.Contains(t, q.Requirements[0], template.subject)
		require.Contains(t, q.Requirements[1], template.object)
		require.Contains(t, q.Requirements[2], template.relation)
		require.Contains(t, q.Requirements[5], template.action)
		prompt := JudgePrompt(q)
		for _, requirement := range q.Requirements {
			require.Contains(t, prompt, requirement)
		}
		if template.object != "自行车" {
			require.NotContains(t, prompt, "自行车")
		}
	}
}

func TestQuestionsHashBindsRubricAndIdentity(t *testing.T) {
	q := Generate("hash-binding")[2]
	original := q.Hash
	q.Requirements[5] = "changed rubric"
	require.NotEqual(t, original, questionHash(q))
	q = Generate("hash-binding")[2]
	q.TemplateID = "other-template"
	require.NotEqual(t, original, questionHash(q))
	ids := AnimationTemplateIDs()
	ids[0] = "tampered"
	require.Equal(t, "pelican-bicycle-v1", AnimationTemplateIDs()[0])
}
