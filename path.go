package tmpl

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
)

type pathItem struct {
	val  reflect.Value
	name string
}

type path []pathItem

func pathRootedAt(v interface{}) path {
	return path{pathItem{
		name: "/",
		val:  reflect.ValueOf(v),
	}}
}

func (p path) String() string {
	if len(p) == 1 {
		return "/."
	}
	buf := bytes.NewBufferString("/")
	for _, it := range p[1:] {
		fmt.Fprintf(buf, ".%s", it.name)
	}
	return buf.String()
}

func (p path) StringWith(its []string) string {
	if len(p) == 1 {
		return fmt.Sprintf("/.%s", strings.Join(its, "."))
	}
	return fmt.Sprintf("%s.%s", p, strings.Join(its, "."))
}

func (p path) itemBehind(num int) (i pathItem, err error) {
	if num < 0 || num >= len(p) {
		err = fmt.Errorf("%q can't pop %d items off", p, num)
		return
	}
	i = p[len(p)-(num+1)]
	return
}

func (p *path) push(i pathItem) {
	*p = append(*p, i)
}

func (p *path) pop(num int) (err error) {
	if num < 0 || num >= len(*p) {
		err = fmt.Errorf("%q cant pop %d items off", p, num)
		return
	}
	*p = (*p)[:len(*p)-(num)]
	return
}

func (p path) lastValue() reflect.Value {
	return p[len(p)-1].val
}

func (p path) dup() (d path) {
	d = make(path, len(p))
	copy(d, p)
	return
}

func (p *path) cd(keys []string, set map[string]reflect.Value) error {
	for _, key := range keys {
		val, err := access(*p, p.lastValue(), key, set)
		if err != nil {
			return err
		}

		p.push(pathItem{
			name: key,
			val:  val,
		})
	}
	return nil
}

func (p path) valueAt(keys []string, set map[string]reflect.Value) (v reflect.Value, err error) {
	v = p.lastValue()
	for i, key := range keys {
		v, err = access(p, v, key, set)
		if err != nil {
			return v, fmt.Errorf("%s%s: Error accessing item %d: %q", p, strings.Join(keys[:i+1], "."), i, key)
		}
	}
	return
}
