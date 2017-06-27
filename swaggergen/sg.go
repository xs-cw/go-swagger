package swaggergen

import (
	"go/ast"
	"strings"

	"github.com/wzshiming/ffmt"
	"github.com/wzshiming/go-swagger/swagger"
	"github.com/wzshiming/gowalk"
)

var ff = ffmt.NewOptional(10, ffmt.StlyeP, ffmt.CanDefaultString|ffmt.CanFilterDuplicate|ffmt.CanRowSpan)

func GenerateHead(rootapi *swagger.Swagger, comments []*ast.CommentGroup) (error error) {

	rootapi.SwaggerVersion = "2.0"

	for _, c := range comments {
		for _, s := range strings.Split(c.Text(), "\n") {
			if strings.HasPrefix(s, "@APIVersion") {
				rootapi.Infos.Version = strings.TrimSpace(s[len("@APIVersion"):])
			} else if strings.HasPrefix(s, "@Title") {
				rootapi.Infos.Title = strings.TrimSpace(s[len("@Title"):])
			} else if strings.HasPrefix(s, "@Description") {
				rootapi.Infos.Description = strings.TrimSpace(s[len("@Description"):])
			} else if strings.HasPrefix(s, "@TermsOfServiceUrl") {
				rootapi.Infos.TermsOfService = strings.TrimSpace(s[len("@TermsOfServiceUrl"):])
			} else if strings.HasPrefix(s, "@Contact") {
				rootapi.Infos.Contact.EMail = strings.TrimSpace(s[len("@Contact"):])
			} else if strings.HasPrefix(s, "@Name") {
				rootapi.Infos.Contact.Name = strings.TrimSpace(s[len("@Name"):])
			} else if strings.HasPrefix(s, "@URL") {
				rootapi.Infos.Contact.URL = strings.TrimSpace(s[len("@URL"):])
			} else if strings.HasPrefix(s, "@LicenseUrl") {
				if rootapi.Infos.License == nil {
					rootapi.Infos.License = &swagger.License{URL: strings.TrimSpace(s[len("@LicenseUrl"):])}
				} else {
					rootapi.Infos.License.URL = strings.TrimSpace(s[len("@LicenseUrl"):])
				}
			} else if strings.HasPrefix(s, "@License") {
				if rootapi.Infos.License == nil {
					rootapi.Infos.License = &swagger.License{Name: strings.TrimSpace(s[len("@License"):])}
				} else {
					rootapi.Infos.License.Name = strings.TrimSpace(s[len("@License"):])
				}
			} else if strings.HasPrefix(s, "@Schemes") {
				rootapi.Schemes = strings.Split(strings.TrimSpace(s[len("@Schemes"):]), ",")
			} else if strings.HasPrefix(s, "@Host") {
				rootapi.Host = strings.TrimSpace(s[len("@Host"):])
			} else if strings.HasPrefix(s, "@BasePath") {
				rootapi.BasePath = strings.TrimSpace(s[len("@BasePath"):])
			}
		}
	}

	return
}

func GB(rootapi *swagger.Swagger, curpath string) {
	routers := gowalk.NewWalk(curpath)

	// 解析头
	ps := routers.Value()
	if sp, ok := ps.(map[string]*ast.Package); ok {
		for _, v := range sp {
			for _, v2 := range v.Files {
				GenerateHead(rootapi, v2.Comments)
			}
		}
	}

	// 解析内容
	controllers := routers.Child("controllers")
	all := controllers.ChildList()
	m := map[string][]string{}
	for _, v := range all {
		i := strings.Index(v, ":")
		if i != -1 && gowalk.IsExported(v) {
			k2 := v[:i]
			v2 := v[i+1:]
			m[k2] = append(m[k2], v2)
		}
	}

	for k, v := range m {
		typ := controllers.Child(k)

		t := typ.Tars()

		rou := ""
		if len(t) >= 3 {
			cg := t[3].(*ast.GenDecl)
			typdoc := cg.Doc.Text()

			d := ParseAtRows(typdoc)

			//ffmt.Puts(fun.Decl)

			if len(d["router"]) != 0 {
				rou = d["router"][0]
			}

			des := ""
			if len(d["description"]) != 0 {
				des = d["description"][0]
			}

			rootapi.Tags = append(rootapi.Tags, swagger.Tag{
				Name:        rou,
				Description: des,
			})
		}
		if rou == "" {
			continue
		}

		for _, v2 := range v {
			fun := controllers.Child(k + ":" + v2)
			GenerateFunc(rootapi, controllers, rou, fun.Doc().Text())
		}
	}
	//ffmt.Puts(m)

}

func GenerateSchema(typname string, node *gowalk.Node) (schema swagger.Schema, message string) {
	if schema.Properties == nil {
		schema.Properties = map[string]swagger.Propertie{}
		schema.Title = typname
		schema.Type = "object"
	}

	ms := []string{}
	cl := node.ChildList()
	for _, v := range cl {
		c := node.Child(v)
		t := c.Type()
		tn := t.Name()
		if tn == "" {
			continue
		}

		n, ok := basicTypes[tn]
		// ffmt.Mark(t.Name())
		if ok {
			ct := strings.Replace(c.Comment().Text(), "\n", " ", -1)
			bb := strings.SplitN(n, ":", 3)
			schema.Properties[c.Name()] = swagger.Propertie{
				Type:        bb[0],
				Format:      bb[1],
				Description: ct,
				Example:     bb[2],
			}

			ms = append(ms, c.Name()+": "+ct)
		} else {
			ffmt.Mark("未定义类型", tn)
			//			v := c.Child(t.Name())
			//			ffmt.P(v.Pos(), t.Name())
		}

		//ffmt.Puts(c.Pos(), c.Name(), c.Type().Name(), c.Comment())
	}

	return schema, strings.Join(ms, "<br/>")
}

// 解析函数
func GenerateFunc(rootapi *swagger.Swagger, node *gowalk.Node, baseurl string, fundoc string) {

	d := ParseAtRows(fundoc)

	//ffmt.Puts(fun.Decl)

	rou := ""
	if len(d["router"]) != 0 {
		rou = d["router"][0]
	}

	// 解析出路由
	ds := parseRouter.FindStringSubmatch(rou)
	if len(ds) == 0 {
		return
	}
	ur := ds[1]
	met := ds[2]
	k := baseurl + ur

	if rootapi.Definitions == nil {
		rootapi.Definitions = map[string]swagger.Schema{}
	}

	// 解析参数
	pars := []swagger.Parameter{}
	for _, v := range d["param"] {
		ps := parseParam.FindStringSubmatch(v)
		if len(d) < 6 {
			continue
		}

		typname := ps[3]
		tp := node.Child(typname)

		ms := ""
		rootapi.Definitions[typname], ms = GenerateSchema(typname, tp)

		//		b := map[string]swagger.Propertie{}
		//		tpcl := tp.ChildList()
		//		for _, v := range tpcl {
		//			c := tp.Child(v)
		//			ffmt.Mark(c.Name())

		//			b[c.Name()] = swagger.Propertie{
		//				Type: "string",
		//				//Format: "int64",
		//			}
		//		}

		//		rootapi.Definitions[typname] = swagger.Schema{
		//			//Ref         string               `json:"$ref,omitempty" yaml:"$ref,omitempty"`
		//			Title: typname,
		//			//Format      string               `json:"format,omitempty" yaml:"format,omitempty"`
		//			//Description string               `json:"description,omitempty" yaml:"description,omitempty"`
		//			//Required    []string             `json:"required,omitempty" yaml:"required,omitempty"`
		//			Type: "object",
		//			//Items       *Schema              `json:"items,omitempty" yaml:"items,omitempty"`
		//			Properties: b,
		//		}
		par := swagger.Parameter{
			In:          ps[1],
			Name:        ps[2],
			Description: ps[5] + "<br/>" + ms,
			Required:    ps[4] == "true",
			Schema: &swagger.Schema{
				Ref: "#/definitions/" + ps[3],
			},
			//Type: ps[3],
			//	Format      string          `json:"format,omitempty" yaml:"format,omitempty"`
			//	Items       *ParameterItems `json:"items,omitempty" yaml:"items,omitempty"`
			//	Default     interface{}     `json:"default,omitempty" yaml:"default,omitempty"`
		}
		pars = append(pars, par)
	}

	resps := map[string]swagger.Response{}
	for _, v := range append(d["success"], d["failure"]...) {
		d := parseResp.FindStringSubmatch(v)
		rr := swagger.Response{}
		if len(d) >= 3 {
			rr.Description = d[2]
		}
		if len(d) >= 4 {
			typname := d[3]
			tp := node.Child(typname)

			ms := ""
			rootapi.Definitions[typname], ms = GenerateSchema(typname, tp)
			if d[3] != "" {
				rr.Schema = &swagger.Schema{
					Ref: "#/definitions/" + d[3],
				}
			}
			rr.Description += "<br/>" + ms
		}
		if len(d) >= 2 {
			resps[d[1]] = rr
		}
	}

	// 解析描述
	desc := ""
	if len(d["description"]) != 0 {
		desc = d["description"][0]
	}

	// 解析标题
	summary := ""
	if len(d["summary"]) != 0 {
		summary = d["summary"][0]
	} else if len(d["title"]) != 0 {
		summary = d["title"][0]
	}

	// 是否禁用
	deprecated := ""
	if len(d["deprecated"]) != 0 {
		deprecated = d["deprecated"][0]
	}

	//	ffmt.Puts(ps)
	ope := &swagger.Operation{
		Tags:        []string{baseurl},
		Summary:     summary,
		Description: desc,
		OperationID: k,
		Consumes:    []string{"application/json"},
		Produces:    []string{"application/json"},
		//Schemes :    []string{},
		Parameters: pars,
		Responses:  resps,
		Deprecated: deprecated == "true",
	}

	if rootapi.Paths == nil {
		rootapi.Paths = map[string]*swagger.Item{}
	}

	if rootapi.Paths[k] == nil {
		rootapi.Paths[k] = &swagger.Item{}
	}

	switch met {
	case "get":
		rootapi.Paths[k].Get = ope
	case "put":
		rootapi.Paths[k].Put = ope
	case "post":
		rootapi.Paths[k].Post = ope
	case "delete":
		rootapi.Paths[k].Delete = ope
	case "options":
		rootapi.Paths[k].Options = ope
	case "head":
		rootapi.Paths[k].Head = ope
	case "patch":
		rootapi.Paths[k].Patch = ope
	}
	//ffmt.Puts(ds)
}