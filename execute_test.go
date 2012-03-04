package tmpl

import (
	"bytes"
	"testing"
)

func TestExecuteNoContext(t *testing.T) {
	cases := []struct {
		templ  string
		expect string
	}{
		{`this is just a literal`, `this is just a literal`},
		{`{% if 1 %}test{% end if %}`, `test`},
		{`{% if 1 %}test{% else %}fail{% end if %}`, `test`},
		{`{% block foo %}test{% end block %}`, `test`},
		{`t{%%}e{%%}s{%%}t{%%}`, `test`},
		{`{% if 0 %}fail{% else %}test{% end if %}`, `test`},
		{`{% if 0 %}fail{% end if %}`, ``},
	}
	for _, c := range cases {
		tree, err := parse(lex([]byte(c.templ)))
		if err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if err := tree.Execute(&buf, nil); err != nil {
			t.Fatal(err)
		}
		if g := buf.String(); g != c.expect {
			t.Fatalf("\nGot %q\nExp %q", g, c.expect)
		}
	}
}

func TestExecuteIfConstanVal(t *testing.T) {
	var sentinal executer = intValue(2)
	cases := []*executeIf{
		{intValue(1), sentinal, nil},
		{floatValue(1), sentinal, nil},
		{constantValue(`foo`), sentinal, nil},
		{intValue(0), nil, sentinal},
		{floatValue(0), nil, sentinal},
		{constantValue(``), nil, sentinal},
	}
	for _, i := range cases {
		if e, isConst := i.constValue(); !isConst || e != sentinal {
			t.Fatal("Expected const sentinal on", i)
		}
	}
}

func TestExecuteListSubstituteIf(t *testing.T) {
	var sentinal executer = intValue(2)
	e := executeList{
		&executeIf{intValue(1), sentinal, nil},
		&executeIf{floatValue(1), sentinal, nil},
		&executeIf{constantValue(`foo`), sentinal, nil},
		&executeIf{intValue(0), nil, sentinal},
		&executeIf{floatValue(0), nil, sentinal},
		&executeIf{constantValue(``), nil, sentinal},
	}
	b := len(e)
	e.substituteTrueIf()
	if b != len(e) {
		t.Fatalf("Lost some items %d -> %d", b, len(e))
	}
	for idx, ex := range e {
		if i, ok := ex.(intValue); !ok || i != sentinal {
			t.Errorf("item %d fails", idx)
		}
	}
}

func TestExecuteListCombineConstant(t *testing.T) {
	e := executeList{
		constantValue(`foo`),
		constantValue(`bar`),
		constantValue(`baz`),
		intValue(2),
		constantValue(`foo`),
		constantValue(`bar`),
		constantValue(`baz`),
	}
	e.combineConstant()
	if len(e) != 3 {
		t.Fatal("Expected 3. Got %d\n%v", len(e), e)
	}
	if g := string(e[0].(constantValue)); g != `foobarbaz` {
		t.Errorf("Value incorrect. %q", g)
	}
	if g := int64(e[1].(intValue)); g != 2 {
		t.Errorf("Value incorrect. %q", g)
	}
	if g := string(e[2].(constantValue)); g != `foobarbaz` {
		t.Errorf("Value incorrect. %q", g)
	}

}