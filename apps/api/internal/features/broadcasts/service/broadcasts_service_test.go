package service

import (
	"slices"
	"testing"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/broadcasts/model"
)

func TestVariablesRenderWithFallbacks(t *testing.T) {
	vs := model.Variables{
		Header: &model.Variable{Source: model.SourceText, Value: "Sale"},
		Body: []model.Variable{
			{Source: model.SourceName, Fallback: "there"},
			{Source: model.SourcePhone},
			{Source: model.SourceAttribute, Value: "city", Fallback: "your city"},
			{Source: model.SourceText, Value: "SAVE10"},
		},
	}
	header, body := vs.Render(model.Contact{Name: "Grace", Phone: "+447700900001", Attributes: map[string]string{"city": "Dhaka"}})
	if header != "Sale" || !slices.Equal(body, []string{"Grace", "+447700900001", "Dhaka", "SAVE10"}) {
		t.Errorf("render = %q %q", header, body)
	}
	_, body = vs.Render(model.Contact{Name: "  ", Phone: "+447700900002"})
	if !slices.Equal(body, []string{"there", "+447700900002", "your city", "SAVE10"}) {
		t.Errorf("fallbacks = %q", body)
	}
}

func TestCheckVariablesNamesWhatIsMissing(t *testing.T) {
	tpl := model.Template{HeaderVariable: true, BodyVariables: 3}
	got := checkVariables(tpl, model.Variables{Body: []model.Variable{
		{Source: model.SourceName},
		{Source: model.SourceText, Value: " "},
	}})
	var fields []string
	for _, d := range got {
		fields = append(fields, d.Field)
	}
	want := []string{"variables.header", "variables.body[0].fallback", "variables.body[1]", "variables.body[2]"}
	if !slices.Equal(fields, want) {
		t.Errorf("missing = %v, want %v", fields, want)
	}
	if got := checkVariables(model.Template{BodyVariables: 1}, model.Variables{Body: []model.Variable{{Source: model.SourcePhone}}}); len(got) != 0 {
		t.Errorf("phone needs no fallback: %v", got)
	}
}

func TestCleanRefusesBadInput(t *testing.T) {
	_, err := clean(model.Input{Name: " ", Variables: model.Variables{Body: []model.Variable{
		{Source: "email"}, {Source: model.SourceAttribute},
	}}})
	if err == nil {
		t.Fatal("bad input accepted")
	}
	in, err := clean(model.Input{Name: "  Spring  ", Variables: model.Variables{Body: []model.Variable{{Source: model.SourceText, Value: " Hi "}}}})
	if err != nil || in.Name != "Spring" || in.Variables.Body[0].Value != "Hi" {
		t.Errorf("clean = %+v, %v", in, err)
	}
}
