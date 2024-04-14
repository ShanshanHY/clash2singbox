package convert

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/xmdhs/clash2singbox/model/clash"
	"github.com/xmdhs/clash2singbox/model/singbox"
)

func filter(isinclude bool, reg string, sl []string) ([]string, error) {
	r, err := regexp.Compile(reg)
	if err != nil {
		return sl, fmt.Errorf("filter: %w", err)
	}
	return getForList(sl, func(v string) (string, bool) {
		has := r.MatchString(v)
		if has && isinclude {
			return v, true
		}
		if !isinclude && !has {
			return v, true
		}
		return "", false
	}), nil
}

func getForList[K, V any](l []K, check func(K) (V, bool)) []V {
	sl := make([]V, 0, len(l))
	for _, v := range l {
		s, ok := check(v)
		if !ok {
			continue
		}
		sl = append(sl, s)
	}
	return sl
}

// func getServers(s []singbox.SingBoxOut) []string {
// 	m := map[string]struct{}{}
// 	return getForList(s, func(v singbox.SingBoxOut) (string, bool) {
// 		server := v.Server
// 		_, has := m[server]
// 		if server == "" || has {
// 			return "", false
// 		}
// 		m[server] = struct{}{}
// 		return server, true
// 	})
// }

func getTags(s []singbox.SingBoxOut) []string {
	return getForList(s, func(v singbox.SingBoxOut) (string, bool) {
		tag := v.Tag
		if tag == "" || v.Ignored {
			return "", false
		}
		return tag, true
	})
}

func Patch(b []byte, s []singbox.SingBoxOut, include, exclude string, group string, extOut []interface{}, extags ...string) ([]byte, error) {
	d, err := PatchMap(b, s, include, exclude, group, extOut, extags, true)
	if err != nil {
		return nil, fmt.Errorf("Patch: %w", err)
	}
	bw := &bytes.Buffer{}
	jw := json.NewEncoder(bw)
	jw.SetIndent("", "    ")
	err = jw.Encode(d)
	if err != nil {
		return nil, fmt.Errorf("Patch: %w", err)
	}
	return bw.Bytes(), nil
}

func ToInsecure(c *clash.Clash) {
	for i := range c.Proxies {
		p := c.Proxies[i]
		p.SkipCertVerify = true
		c.Proxies[i] = p
	}
}

func PatchMap(
	tpl []byte,
	s []singbox.SingBoxOut,
	include, exclude string,
	group string,
	extOut []interface{},
	extags []string,
	urltestOut bool,
) (map[string]any, error) {
	d := map[string]interface{}{}
	err := json.Unmarshal(tpl, &d)
	if err != nil {
		return nil, fmt.Errorf("PatchMap: %w", err)
	}
	tags := getTags(s)

	tags = append(tags, extags...)

	ftags := tags
	if include != "" {
		ftags, err = filter(true, include, ftags)
		if err != nil {
			return nil, fmt.Errorf("PatchMap: %w", err)
		}
	}
	if exclude != "" {
		ftags, err = filter(false, exclude, ftags)
		if err != nil {
			return nil, fmt.Errorf("PatchMap: %w", err)
		}
	}

	var sSelect, sUrltest []singbox.SingBoxOut

	if urltestOut {
		sSelect = append(sSelect, singbox.SingBoxOut{
			Type:      "selector",
			Tag:       "select",
			Outbounds: append([]string{"urltest"}, tags...),
			Default:   "urltest",
		})
		sUrltest = append(sUrltest, singbox.SingBoxOut{
			Type:      "urltest",
			Tag:       "urltest",
			Outbounds: ftags,
		})
	}

	if group != "" {
		groups := strings.Split(group, ",")
		for _, g := range groups {

			n, m := GroupFilter(g, ftags)
			if n == "" || len(m) == 0 {
				continue
			}

			sSelect = append(sSelect, singbox.SingBoxOut{
				Type:      "selector",
				Tag:       n,
				Outbounds: append([]string{fmt.Sprintf("%s-urltest", n)}, m...),
				Default:   fmt.Sprintf("%s-urltest", n),
			})
			sUrltest = append(sUrltest, singbox.SingBoxOut{
				Type:      "urltest",
				Tag:       fmt.Sprintf("%s-urltest", n),
				Outbounds: m,
			})
		}
	}

	s = append(append(sSelect, s...), sUrltest...)

	s = append(s, singbox.SingBoxOut{
		Type: "direct",
		Tag:  "direct",
	})
	s = append(s, singbox.SingBoxOut{
		Type: "block",
		Tag:  "block",
	})
	s = append(s, singbox.SingBoxOut{
		Type: "dns",
		Tag:  "dns-out",
	})

	anyList := make([]any, 0, len(s)+len(extOut))
	for _, v := range s {
		anyList = append(anyList, v)
	}
	anyList = append(anyList, extOut...)

	d["outbounds"] = anyList

	return d, nil
}

func GroupFilter(g string, tags []string) (GroupName string, GroupMember []string) {

	gFilter := strings.Split(g, ":")
	if len(gFilter) != 2 {
		fmt.Printf("错误的组过滤格式输入: %s\n", g)
		return
	}

	f, err := regexp.Compile(gFilter[1])
	if err != nil {
		fmt.Printf("错误的过滤正则表达式: %s\n", gFilter[1])
		return
	}

	for _, t := range tags {
		if f.MatchString(t) {
			GroupMember = append(GroupMember, t)
		}
	}
	return gFilter[0], GroupMember
}
