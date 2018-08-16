package main

import (
	"fmt"
	"github.com/go-sql-driver/mysql"
	"github.com/go-xorm/xorm"
	"io/ioutil"
	"os"
	"path"
	"strings"
)

var (
	engine *xorm.Engine
)

func CheckErr(err error) {
	if err != nil {
		panic(err)
	}
}

func ToCamelCase(underscore string) string {
	s := strings.ToLower(underscore)
	s = strings.Replace(s, "_", " ", -1)
	s = strings.Title(s)
	s = strings.Replace(s, " ", "", -1)
	return s
}

func Fullpath(p string) string {
	p = path.Clean(p)
	if strings.HasPrefix(p, "/") {
		return p
	}
	pwd, err := os.Getwd()
	CheckErr(err)
	return path.Clean(pwd + "/" + p)
}

func ParseColumn(column map[string][]byte) []string {
	field := ToCamelCase(string(column["Field"]))

	prefixMap := [][]string{
		{"int", "int"},
		{"smallint", "int"},
		{"bigint", "int64"},
		{"float", "float32"},
		{"double", "float64"},
		{"char", "string"},
		{"blob", "[]uint8"},
		{"varchar", "string"},
		{"text", "string"},
		{"bool", "bool"},
		{"timestamp", "time.Time"},
		{"date", "time.Time"},
		{"datetime", "time.Time"},
		{"enum", "string"},
	}

	tp := strings.ToLower(string(column["Type"]))
	sql_type := "varchar"
	go_type := "string"

	for _, item := range prefixMap {
		prefix, tp_candidate := item[0], item[1]
		if strings.HasPrefix(tp, prefix) {
			go_type = tp_candidate
			sql_type = prefix
			break
		}
	}

	var tags []string

	switch string(column["Key"]) {
	case "PRI":
		tags = append(tags, "pk")
	case "UNI":
		tags = append(tags, "unique")
	case "MUL":
		tags = append(tags, "index")
	}

	if sql_type == "char" || sql_type == "varchar" {
		tags = append(tags, tp)
	}

	extra := string(column["Extra"])
	if strings.Contains(extra, "auto_increment") {
		tags = append(tags, "autoincr")
	}

	if string(column["Null"]) == "NO" {
		tags = append(tags, "notnull")
	} else {
		if go_type == "string" {
			go_type = "sql.NullString"
		}
	}

	default_value := string(column["Default"])
	if default_value != "" {
		if go_type == "string" || go_type == "sql.NullString" {
			default_value = "'" + default_value + "'"
		}
		tags = append(tags, "default("+default_value+")")
	}

	if field == "CreatedAt" {
		tags = append(tags, "created")
	}

	if field == "UpdatedAt" {
		tags = append(tags, "updated")
	}

	if field == "DeletedAt" {
		tags = append(tags, "deleted")
	}

	tag := ""
	if len(tags) > 0 {
		tag = "`orm:\"" + strings.Join(tags, " ") + "\"`"
	}

	return []string{field, go_type, tag}
}

func TableToStruct(TableName string) string {
	code := "type " + ToCamelCase(TableName) + " struct {\n"

	sql := "desc `" + TableName + "`"
	results, err := engine.Query(sql)
	CheckErr(err)

	columns := [][]string{}
	maxlen := []int{0, 0, 0}
	for _, column := range results {
		result := ParseColumn(column)
		for i, v := range result {
			if maxlen[i] < len(v) {
				maxlen[i] = len(v)
			}
		}
		columns = append(columns, result)
	}

	for _, column := range columns {
		code += fmt.Sprintf("    %-*s %-*s %-*s\n", maxlen[0], column[0], maxlen[1], column[1], maxlen[2], column[2])
	}

	return code + "}\n"
}

func ShouldGenerate(tables []string, table string) bool {
	if len(tables) == 0 {
		return true
	}
	for _, t := range tables {
		if table == t {
			return true
		}
	}
	return false
}

func DbGenerator(dsn, dirname string, tables []string) {
	var err error

	fullpath := Fullpath(os.Args[2])
	stat, err := os.Stat(fullpath)
	if err != nil {
		fmt.Printf("Invalid path(%s): %s", dirname, err.Error())
		os.Exit(1)
	}
	if !stat.IsDir() {
		fmt.Printf("Invalid path(%s): not a directory", dirname)
		os.Exit(1)
	}

	package_name := path.Base(fullpath)

	if engine == nil {
		engine, err = xorm.NewEngine("mysql", dsn)
		CheckErr(err)
	}

	config, err := mysql.ParseDSN(dsn)
	CheckErr(err)

	results, err := engine.Query("show tables")
	CheckErr(err)

	for _, table := range results {
		TableName := string(table["Tables_in_"+config.DBName])
		if !ShouldGenerate(tables, TableName) {
			fmt.Printf("[Skip] %s\n", TableName)
			continue
		}
		code := TableToStruct(TableName)

		section_package := "package " + package_name + "\n\n"

		section_import := ""
		if strings.Contains(code, "sql.NullString") {
			section_import = "import \"database/sql\"\n\n"
		}

		code = section_package + section_import + code

		filename := fmt.Sprintf("%s/%s.go", fullpath, TableName)

		fmt.Printf("[Generate] %s: %s\n%s\n", TableName, filename, code)
		err = ioutil.WriteFile(filename, []byte(code), 0644)
		CheckErr(err)
	}
}

func main() {
	if len(os.Args) < 3 {
		fmt.Printf("Usage:\n  %s <dsn> <dir path> [table_list]\n\n", os.Args[0])
		fmt.Printf("Example:\n  %s \"root:123456@(127.0.0.1:3306)/test\" ./models \"user,address\"\n\n", os.Args[0])
		return
	}

	tables := []string{}
	if len(os.Args) >= 4 {
		tables = strings.Split(os.Args[3], ",")
	}

	//dsn := "root:123456@(127.0.0.1:3306)/test?charset=utf8"
	DbGenerator(os.Args[1], os.Args[2], tables)
}
