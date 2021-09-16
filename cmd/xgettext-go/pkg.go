// Copyright 2020 ChaiShushan <chaishushan{AT}gmail.com>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/chai2010/gettext-go/po"
)

type Package struct {
	path         string
	pkgpath      string
	pkgname      string
	filesAbspath []string

	fset         *token.FileSet
	astFiles     []*ast.File
	typesInfo    *types.Info
	typesPackage *types.Package

	potFile *po.File
}

func LoadPackage(path string) *Package {
	p := &Package{
		path:         path,
		pkgpath:      gopkgPath(path),
		pkgname:      gopkgName(path),
		filesAbspath: gopkgFilesAbspath(path),
	}

	var fset = token.NewFileSet()
	var astFiles = make([]*ast.File, len(p.filesAbspath))

	for i, path := range p.filesAbspath {
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			log.Fatal(err)
		}
		astFiles[i] = f
	}

	// https://github.com/golang/go/issues/26504
	typesConfig := &types.Config{
		Importer:    importer.For("source", nil),
		FakeImportC: true,
	}
	typesInfo := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
	}

	typesPackage, err := typesConfig.Check(p.pkgname, fset, astFiles, typesInfo)
	if err != nil {
		log.Fatal(err)
	}

	p.fset = fset
	p.astFiles = astFiles
	p.typesInfo = typesInfo
	p.typesPackage = typesPackage

	return p
}

func (p *Package) GenPotFile() *po.File {
	p.potFile = &po.File{
		MimeHeader: po.Header{
			Comment: po.Comment{
				TranslatorComment: "" +
					fmt.Sprintf("package: %s\n\n", p.pkgpath) +
					"Generated By gettext-go" + "\n" +
					"https://github.com/chai2010/gettext-go" + "\n",
			},
			ProjectIdVersion:        "1.0",
			POTCreationDate:         time.Now().Format("2006-01-02 15:04-0700"),
			LanguageTeam:            "golang-china",
			Language:                "zh_CN",
			MimeVersion:             "MIME-Version: 1.0",
			ContentType:             "Content-Type: text/plain; charset=UTF-8",
			ContentTransferEncoding: "Content-Transfer-Encoding: 8bit",
		},
	}

	for _, f := range p.astFiles {
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.CallExpr:
				switch sel := x.Fun.(type) {
				case *ast.SelectorExpr:
					p.processGettext(x, sel.Sel.Name)
				}
			}
			return true
		})
	}

	return p.potFile
}

func (p *Package) processGettext(x *ast.CallExpr, fnName string) {
	switch fnName {
	case "Gettext": // Gettext(msgid string) string
		pos := p.fset.Position(x.Pos())
		p.potFile.Messages = append(p.potFile.Messages, po.Message{
			Comment: po.Comment{
				ReferenceFile: []string{p.pkgpath + "/" + filepath.Base(pos.Filename)},
				ReferenceLine: []int{pos.Line},
				Flags:         []string{"go-format"},
			},
			MsgContext: "",
			MsgId:      p.evalStringValue(x.Args[0]),
		})

	case "PGettext": // PGettext(msgctxt, msgid string) string
		pos := p.fset.Position(x.Pos())
		p.potFile.Messages = append(p.potFile.Messages, po.Message{
			Comment: po.Comment{
				ReferenceFile: []string{p.pkgpath + "/" + filepath.Base(pos.Filename)},
				ReferenceLine: []int{pos.Line},
				Flags:         []string{"go-format"},
			},
			MsgContext: p.evalStringValue(x.Args[0]),
			MsgId:      p.evalStringValue(x.Args[1]),
		})

	case "NGettext": // NGettext(msgid, msgidPlural string, n int) string
		// TODO
	case "PNGettext": // PNGettext(msgctxt, msgid, msgidPlural string, n int) string
		// TODO

	case "DGettext": // DGettext(domain, msgid string) string
		// TODO
	case "DPGettext": // DPGettext(domain, msgctxt, msgid string) string
		// TODO
	case "DNGettext": // DNGettext(domain, msgid, msgidPlural string, n int) string
		// TODO
	case "DPNGettext": // DPNGettext(domain, msgctxt, msgid, msgidPlural string, n int) string
		// TODO
	}
}

func (p *Package) evalStringValue(val interface{}) string {
	switch val.(type) {
	case *ast.BasicLit:
		s := val.(*ast.BasicLit).Value
		s = strings.TrimSpace(s)
		s = strings.Trim(s, `"`+"\n")
		return s
	case *ast.BinaryExpr:
		if val.(*ast.BinaryExpr).Op != token.ADD {
			return ""
		}
		left := p.evalStringValue(val.(*ast.BinaryExpr).X)
		right := p.evalStringValue(val.(*ast.BinaryExpr).Y)
		return left[0:len(left)-1] + right[1:len(right)]
	default:
		panic(fmt.Sprintf("unknown type: %+v", val))
	}
}