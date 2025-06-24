package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// データ型の定義
type DataType string

const (
	TypeInteger DataType = "INTEGER"
	TypeVarchar DataType = "VARCHAR"
	TypeBoolean DataType = "BOOLEAN"
)

// カラム定義
type Column struct {
	Name    string   `json:"name"`
	Type    DataType `json:"type"`
	Size    int      `json:"size,omitempty"` // VARCHAR用
	NotNull bool     `json:"not_null"`
	Primary bool     `json:"primary"`
}

// テーブル定義
type Table struct {
	Name    string   `json:"name"`
	Columns []Column `json:"columns"`
	Rows    []Row    `json:"rows"`
}

// 行データ
type Row map[string]interface{}

// データベース
type Database struct {
	Name   string            `json:"name"`
	Tables map[string]*Table `json:"tables"`
	dbPath string
}

// クエリ結果
type QueryResult struct {
	Columns []string
	Rows    []Row
	Message string
	Error   error
}

// WHERE条件
type WhereCondition struct {
	Column   string
	Operator string
	Value    interface{}
}

// SQLパーサー
type SQLParser struct {
	db *Database
}

// データベース初期化
func NewDatabase(name string) *Database {
	dbPath := fmt.Sprintf("./db_%s", name)
	os.MkdirAll(dbPath, 0755)

	return &Database{
		Name:   name,
		Tables: make(map[string]*Table),
		dbPath: dbPath,
	}
}

// データベース読み込み
func LoadDatabase(name string) (*Database, error) {
	db := NewDatabase(name)

	// メタデータファイルを読み込み
	metaPath := filepath.Join(db.dbPath, "metadata.json")
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		// 新規データベース
		return db, nil
	}

	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(data, db); err != nil {
		return nil, err
	}

	// 各テーブルのデータを読み込み
	for tableName, table := range db.Tables {
		tablePath := filepath.Join(db.dbPath, fmt.Sprintf("%s.json", tableName))
		if data, err := os.ReadFile(tablePath); err == nil {
			var rows []Row
			if err := json.Unmarshal(data, &rows); err == nil {
				table.Rows = rows
			}
		}
	}

	return db, nil
}

// データベース保存
func (db *Database) Save() error {
	// メタデータを保存
	metaPath := filepath.Join(db.dbPath, "metadata.json")
	metaData, err := json.MarshalIndent(map[string]interface{}{
		"name":   db.Name,
		"tables": db.getTableMetadata(),
	}, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(metaPath, metaData, 0644); err != nil {
		return err
	}

	// 各テーブルのデータを保存
	for name, table := range db.Tables {
		tablePath := filepath.Join(db.dbPath, fmt.Sprintf("%s.json", name))
		data, err := json.MarshalIndent(table.Rows, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(tablePath, data, 0644); err != nil {
			return err
		}
	}

	return nil
}

// テーブルメタデータ取得
func (db *Database) getTableMetadata() map[string]interface{} {
	metadata := make(map[string]interface{})
	for name, table := range db.Tables {
		metadata[name] = map[string]interface{}{
			"name":    table.Name,
			"columns": table.Columns,
		}
	}
	return metadata
}

// CREATE TABLE実装
func (db *Database) CreateTable(name string, columns []Column) error {
	if _, exists := db.Tables[name]; exists {
		return fmt.Errorf("table '%s' already exists", name)
	}

	// プライマリキーチェック
	primaryCount := 0
	for _, col := range columns {
		if col.Primary {
			primaryCount++
		}
	}
	if primaryCount > 1 {
		return fmt.Errorf("multiple primary keys defined")
	}

	db.Tables[name] = &Table{
		Name:    name,
		Columns: columns,
		Rows:    []Row{},
	}

	return db.Save()
}

// INSERT実装
func (db *Database) Insert(tableName string, values map[string]interface{}) error {
	table, exists := db.Tables[tableName]
	if !exists {
		return fmt.Errorf("table '%s' does not exist", tableName)
	}

	// データ型チェックと変換
	row := make(Row)
	for _, col := range table.Columns {
		value, exists := values[col.Name]

		// NOT NULL制約チェック
		if col.NotNull && (!exists || value == nil) {
			return fmt.Errorf("column '%s' cannot be null", col.Name)
		}

		// データ型チェック
		if exists && value != nil {
			convertedValue, err := validateAndConvertValue(value, col)
			if err != nil {
				return fmt.Errorf("column '%s': %v", col.Name, err)
			}
			row[col.Name] = convertedValue
		} else {
			row[col.Name] = nil
		}
	}

	// プライマリキーの重複チェック
	for _, col := range table.Columns {
		if col.Primary {
			for _, existingRow := range table.Rows {
				if existingRow[col.Name] == row[col.Name] {
					return fmt.Errorf("duplicate primary key value: %v", row[col.Name])
				}
			}
		}
	}

	table.Rows = append(table.Rows, row)
	return db.Save()
}

// SELECT実装
func (db *Database) Select(tableName string, columns []string, where *WhereCondition) (*QueryResult, error) {
	table, exists := db.Tables[tableName]
	if !exists {
		return nil, fmt.Errorf("table '%s' does not exist", tableName)
	}

	// カラム検証
	selectColumns := columns
	if len(columns) == 1 && columns[0] == "*" {
		selectColumns = []string{}
		for _, col := range table.Columns {
			selectColumns = append(selectColumns, col.Name)
		}
	} else {
		for _, colName := range columns {
			if !table.hasColumn(colName) {
				return nil, fmt.Errorf("column '%s' does not exist", colName)
			}
		}
	}

	// 結果を作成
	result := &QueryResult{
		Columns: selectColumns,
		Rows:    []Row{},
	}

	// 行をフィルタリング
	for _, row := range table.Rows {
		if where != nil {
			match, err := evaluateWhere(row, where)
			if err != nil {
				return nil, err
			}
			if !match {
				continue
			}
		}

		// 選択されたカラムのみを含む行を作成
		selectedRow := make(Row)
		for _, col := range selectColumns {
			selectedRow[col] = row[col]
		}
		result.Rows = append(result.Rows, selectedRow)
	}

	return result, nil
}

// UPDATE実装
func (db *Database) Update(tableName string, updates map[string]interface{}, where *WhereCondition) (int, error) {
	table, exists := db.Tables[tableName]
	if !exists {
		return 0, fmt.Errorf("table '%s' does not exist", tableName)
	}

	// 更新するカラムの検証
	for colName, value := range updates {
		col := table.getColumn(colName)
		if col == nil {
			return 0, fmt.Errorf("column '%s' does not exist", colName)
		}

		// データ型チェック
		if value != nil {
			_, err := validateAndConvertValue(value, *col)
			if err != nil {
				return 0, fmt.Errorf("column '%s': %v", colName, err)
			}
		} else if col.NotNull {
			return 0, fmt.Errorf("column '%s' cannot be null", colName)
		}
	}

	// 更新実行
	updatedCount := 0
	for i, row := range table.Rows {
		if where != nil {
			match, err := evaluateWhere(row, where)
			if err != nil {
				return 0, err
			}
			if !match {
				continue
			}
		}

		// 行を更新
		for colName, value := range updates {
			col := table.getColumn(colName)
			if value != nil {
				convertedValue, _ := validateAndConvertValue(value, *col)
				table.Rows[i][colName] = convertedValue
			} else {
				table.Rows[i][colName] = nil
			}
		}
		updatedCount++
	}

	if err := db.Save(); err != nil {
		return 0, err
	}

	return updatedCount, nil
}

// DELETE実装
func (db *Database) Delete(tableName string, where *WhereCondition) (int, error) {
	table, exists := db.Tables[tableName]
	if !exists {
		return 0, fmt.Errorf("table '%s' does not exist", tableName)
	}

	// 削除対象の行を特定
	newRows := []Row{}
	deletedCount := 0

	for _, row := range table.Rows {
		shouldDelete := false

		if where != nil {
			match, err := evaluateWhere(row, where)
			if err != nil {
				return 0, err
			}
			shouldDelete = match
		} else {
			shouldDelete = true // WHERE句がない場合は全行削除
		}

		if shouldDelete {
			deletedCount++
		} else {
			newRows = append(newRows, row)
		}
	}

	table.Rows = newRows

	if err := db.Save(); err != nil {
		return 0, err
	}

	return deletedCount, nil
}

// ヘルパー関数
func (t *Table) hasColumn(name string) bool {
	for _, col := range t.Columns {
		if col.Name == name {
			return true
		}
	}
	return false
}

func (t *Table) getColumn(name string) *Column {
	for _, col := range t.Columns {
		if col.Name == name {
			return &col
		}
	}
	return nil
}

// データ型検証と変換
func validateAndConvertValue(value interface{}, col Column) (interface{}, error) {
	switch col.Type {
	case TypeInteger:
		switch v := value.(type) {
		case int:
			return v, nil
		case float64:
			return int(v), nil
		case string:
			return strconv.Atoi(v)
		default:
			return nil, fmt.Errorf("invalid integer value")
		}

	case TypeVarchar:
		str := fmt.Sprintf("%v", value)
		if col.Size > 0 && len(str) > col.Size {
			return nil, fmt.Errorf("string too long (max %d)", col.Size)
		}
		return str, nil

	case TypeBoolean:
		switch v := value.(type) {
		case bool:
			return v, nil
		case string:
			return strconv.ParseBool(v)
		default:
			return nil, fmt.Errorf("invalid boolean value")
		}
	}

	return nil, fmt.Errorf("unknown data type")
}

// WHERE条件評価
func evaluateWhere(row Row, where *WhereCondition) (bool, error) {
	value, exists := row[where.Column]
	if !exists {
		return false, fmt.Errorf("column '%s' does not exist", where.Column)
	}

	// NULL値の処理
	if value == nil {
		switch where.Operator {
		case "IS":
			return where.Value == nil, nil
		case "IS NOT":
			return where.Value != nil, nil
		default:
			return false, nil
		}
	}

	// 比較演算
	switch where.Operator {
	case "=":
		return compareValues(value, where.Value) == 0, nil
	case "!=", "<>":
		return compareValues(value, where.Value) != 0, nil
	case ">":
		return compareValues(value, where.Value) > 0, nil
	case ">=":
		return compareValues(value, where.Value) >= 0, nil
	case "<":
		return compareValues(value, where.Value) < 0, nil
	case "<=":
		return compareValues(value, where.Value) <= 0, nil
	case "LIKE":
		return matchLike(fmt.Sprintf("%v", value), fmt.Sprintf("%v", where.Value)), nil
	default:
		return false, fmt.Errorf("unknown operator: %s", where.Operator)
	}
}

// 値の比較
func compareValues(a, b interface{}) int {
	// 数値比較
	aNum, aIsNum := toNumber(a)
	bNum, bIsNum := toNumber(b)
	if aIsNum && bIsNum {
		if aNum < bNum {
			return -1
		} else if aNum > bNum {
			return 1
		}
		return 0
	}

	// 文字列比較
	aStr := fmt.Sprintf("%v", a)
	bStr := fmt.Sprintf("%v", b)
	return strings.Compare(aStr, bStr)
}

func toNumber(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case float64:
		return n, true
	case string:
		if f, err := strconv.ParseFloat(n, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

// LIKE演算子の実装
func matchLike(str, pattern string) bool {
	// % を .* に、_ を . に変換
	pattern = strings.ReplaceAll(pattern, "%", ".*")
	pattern = strings.ReplaceAll(pattern, "_", ".")
	pattern = "^" + pattern + "$"

	matched, _ := regexp.MatchString(pattern, str)
	return matched
}

// SQLパーサー実装
func NewSQLParser(db *Database) *SQLParser {
	return &SQLParser{db: db}
}

func (p *SQLParser) Parse(query string) (*QueryResult, error) {
	query = strings.TrimSpace(query)
	tokens := tokenize(query)

	if len(tokens) == 0 {
		return nil, fmt.Errorf("empty query")
	}

	switch strings.ToUpper(tokens[0]) {
	case "CREATE":
		return p.parseCreate(tokens)
	case "INSERT":
		return p.parseInsert(tokens)
	case "SELECT":
		return p.parseSelect(tokens)
	case "UPDATE":
		return p.parseUpdate(tokens)
	case "DELETE":
		return p.parseDelete(tokens)
	default:
		return nil, fmt.Errorf("unknown command: %s", tokens[0])
	}
}

// トークン化
func tokenize(query string) []string {
	// 簡易的なトークン化（引用符内のスペースを保持）
	var tokens []string
	var current strings.Builder
	inQuote := false
	quoteChar := rune(0)

	for _, r := range query {
		if !inQuote && (r == '\'' || r == '"') {
			inQuote = true
			quoteChar = r
		} else if inQuote && r == quoteChar {
			inQuote = false
			tokens = append(tokens, current.String())
			current.Reset()
		} else if !inQuote && (r == ' ' || r == '\t' || r == '\n' || r == ',') {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			if r == ',' {
				tokens = append(tokens, ",")
			}
		} else if !inQuote && (r == '(' || r == ')' || r == ';') {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			tokens = append(tokens, string(r))
		} else {
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

// CREATE TABLE パース
func (p *SQLParser) parseCreate(tokens []string) (*QueryResult, error) {
	if len(tokens) < 4 || strings.ToUpper(tokens[1]) != "TABLE" {
		return nil, fmt.Errorf("invalid CREATE TABLE syntax")
	}

	tableName := tokens[2]

	// カラム定義をパース
	columns := []Column{}
	i := 4 // '(' の後から開始

	for i < len(tokens) && tokens[i] != ")" {
		if tokens[i] == "," {
			i++
			continue
		}

		// カラム名
		colName := tokens[i]
		i++

		// データ型
		if i >= len(tokens) {
			return nil, fmt.Errorf("missing data type for column %s", colName)
		}

		colType := DataType(strings.ToUpper(tokens[i]))
		i++

		col := Column{
			Name: colName,
			Type: colType,
		}

		// VARCHAR(size)の処理
		if colType == TypeVarchar && i < len(tokens) && tokens[i] == "(" {
			i++
			if i < len(tokens) {
				size, err := strconv.Atoi(tokens[i])
				if err != nil {
					return nil, fmt.Errorf("invalid size for VARCHAR")
				}
				col.Size = size
				i += 2 // size and ')'
			}
		}

		// 制約の処理
		for i < len(tokens) && tokens[i] != "," && tokens[i] != ")" {
			constraint := strings.ToUpper(tokens[i])
			switch constraint {
			case "NOT":
				if i+1 < len(tokens) && strings.ToUpper(tokens[i+1]) == "NULL" {
					col.NotNull = true
					i++
				}
			case "PRIMARY":
				if i+1 < len(tokens) && strings.ToUpper(tokens[i+1]) == "KEY" {
					col.Primary = true
					i++
				}
			}
			i++
		}

		columns = append(columns, col)
	}

	err := p.db.CreateTable(tableName, columns)
	if err != nil {
		return nil, err
	}

	return &QueryResult{
		Message: fmt.Sprintf("Table '%s' created successfully", tableName),
	}, nil
}

// INSERT パース
func (p *SQLParser) parseInsert(tokens []string) (*QueryResult, error) {
	if len(tokens) < 4 || strings.ToUpper(tokens[1]) != "INTO" {
		return nil, fmt.Errorf("invalid INSERT syntax")
	}

	tableName := tokens[2]

	// VALUES句を探す
	valuesIndex := -1
	for i, token := range tokens {
		if strings.ToUpper(token) == "VALUES" {
			valuesIndex = i
			break
		}
	}

	if valuesIndex == -1 {
		return nil, fmt.Errorf("missing VALUES clause")
	}

	// カラム名をパース（オプション）
	var columns []string
	if tokens[3] == "(" {
		i := 4
		for i < valuesIndex && tokens[i] != ")" {
			if tokens[i] != "," {
				columns = append(columns, tokens[i])
			}
			i++
		}
	}

	// 値をパース
	values := make(map[string]interface{})
	i := valuesIndex + 2 // VALUES ( の後
	valueIndex := 0

	table := p.db.Tables[tableName]
	if table == nil {
		return nil, fmt.Errorf("table '%s' does not exist", tableName)
	}

	// カラムが指定されていない場合は、テーブル定義の順序を使用
	if len(columns) == 0 {
		for _, col := range table.Columns {
			columns = append(columns, col.Name)
		}
	}

	for i < len(tokens) && tokens[i] != ")" {
		if tokens[i] == "," {
			i++
			continue
		}

		if valueIndex >= len(columns) {
			return nil, fmt.Errorf("too many values")
		}

		// 値の解析
		value := parseValue(tokens[i])
		values[columns[valueIndex]] = value

		valueIndex++
		i++
	}

	err := p.db.Insert(tableName, values)
	if err != nil {
		return nil, err
	}

	return &QueryResult{
		Message: "1 row inserted",
	}, nil
}

// SELECT パース
func (p *SQLParser) parseSelect(tokens []string) (*QueryResult, error) {
	if len(tokens) < 4 {
		return nil, fmt.Errorf("invalid SELECT syntax")
	}

	// カラムをパース
	columns := []string{}
	i := 1
	for i < len(tokens) && strings.ToUpper(tokens[i]) != "FROM" {
		if tokens[i] != "," {
			columns = append(columns, tokens[i])
		}
		i++
	}

	if strings.ToUpper(tokens[i]) != "FROM" {
		return nil, fmt.Errorf("missing FROM clause")
	}
	i++

	if i >= len(tokens) {
		return nil, fmt.Errorf("missing table name")
	}

	tableName := tokens[i]
	i++

	// WHERE句をパース
	var where *WhereCondition
	if i < len(tokens) && strings.ToUpper(tokens[i]) == "WHERE" {
		i++
		if i+2 < len(tokens) {
			where = &WhereCondition{
				Column:   tokens[i],
				Operator: strings.ToUpper(tokens[i+1]),
				Value:    parseValue(tokens[i+2]),
			}
		}
	}

	return p.db.Select(tableName, columns, where)
}

// UPDATE パース
func (p *SQLParser) parseUpdate(tokens []string) (*QueryResult, error) {
	if len(tokens) < 6 {
		return nil, fmt.Errorf("invalid UPDATE syntax")
	}

	tableName := tokens[1]

	if strings.ToUpper(tokens[2]) != "SET" {
		return nil, fmt.Errorf("missing SET clause")
	}

	// SET句をパース
	updates := make(map[string]interface{})
	i := 3

	for i < len(tokens) && strings.ToUpper(tokens[i]) != "WHERE" {
		if tokens[i] == "," {
			i++
			continue
		}

		colName := tokens[i]
		if i+2 >= len(tokens) || tokens[i+1] != "=" {
			return nil, fmt.Errorf("invalid SET syntax")
		}

		value := parseValue(tokens[i+2])
		updates[colName] = value
		i += 3
	}

	// WHERE句をパース
	var where *WhereCondition
	if i < len(tokens) && strings.ToUpper(tokens[i]) == "WHERE" {
		i++
		if i+2 < len(tokens) {
			where = &WhereCondition{
				Column:   tokens[i],
				Operator: strings.ToUpper(tokens[i+1]),
				Value:    parseValue(tokens[i+2]),
			}
		}
	}

	count, err := p.db.Update(tableName, updates, where)
	if err != nil {
		return nil, err
	}

	return &QueryResult{
		Message: fmt.Sprintf("%d row(s) updated", count),
	}, nil
}

// DELETE パース
func (p *SQLParser) parseDelete(tokens []string) (*QueryResult, error) {
	if len(tokens) < 3 || strings.ToUpper(tokens[1]) != "FROM" {
		return nil, fmt.Errorf("invalid DELETE syntax")
	}

	tableName := tokens[2]

	// WHERE句をパース
	var where *WhereCondition
	if len(tokens) > 3 && strings.ToUpper(tokens[3]) == "WHERE" {
		if len(tokens) >= 7 {
			where = &WhereCondition{
				Column:   tokens[4],
				Operator: strings.ToUpper(tokens[5]),
				Value:    parseValue(tokens[6]),
			}
		}
	}

	count, err := p.db.Delete(tableName, where)
	if err != nil {
		return nil, err
	}

	return &QueryResult{
		Message: fmt.Sprintf("%d row(s) deleted", count),
	}, nil
}

// 値のパース
func parseValue(token string) interface{} {
	// NULL
	if strings.ToUpper(token) == "NULL" {
		return nil
	}

	// Boolean
	if strings.ToUpper(token) == "TRUE" {
		return true
	}
	if strings.ToUpper(token) == "FALSE" {
		return false
	}

	// 数値
	if num, err := strconv.Atoi(token); err == nil {
		return num
	}

	// それ以外は文字列
	return token
}

// 結果表示
func (r *QueryResult) Display() {
	if r.Error != nil {
		fmt.Printf("Error: %v\n", r.Error)
		return
	}

	if r.Message != "" {
		fmt.Println(r.Message)
		return
	}

	if len(r.Rows) == 0 {
		fmt.Println("No rows returned")
		return
	}

	// ヘッダー表示
	fmt.Println(strings.Repeat("-", 80))
	for _, col := range r.Columns {
		fmt.Printf("| %-20s ", col)
	}
	fmt.Println("|")
	fmt.Println(strings.Repeat("-", 80))

	// データ表示
	for _, row := range r.Rows {
		for _, col := range r.Columns {
			value := row[col]
			if value == nil {
				fmt.Printf("| %-20s ", "NULL")
			} else {
				fmt.Printf("| %-20v ", value)
			}
		}
		fmt.Println("|")
	}
	fmt.Println(strings.Repeat("-", 80))

	fmt.Printf("%d row(s) returned\n", len(r.Rows))
}

// メイン関数
func main() {
	fmt.Println("Simple RDBMS - Type 'help' for commands")
	fmt.Println("========================================")

	// データベースを初期化または読み込み
	db, err := LoadDatabase("mydb")
	if err != nil {
		fmt.Printf("Failed to load database: %v\n", err)
		return
	}

	parser := NewSQLParser(db)
	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("\nSQL> ")
		if !scanner.Scan() {
			break
		}

		query := strings.TrimSpace(scanner.Text())

		// 特殊コマンド
		switch strings.ToLower(query) {
		case "exit", "quit":
			fmt.Println("Goodbye!")
			return
		case "help":
			printHelp()
			continue
		case "tables":
			showTables(db)
			continue
		case "":
			continue
		}

		// SQL実行
		result, err := parser.Parse(query)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		} else {
			result.Display()
		}
	}
}

// ヘルプ表示
func printHelp() {
	fmt.Println(`
Commands:
  CREATE TABLE table_name (column_name data_type [constraints], ...)
  INSERT INTO table_name [(columns)] VALUES (values)
  SELECT columns FROM table_name [WHERE condition]
  UPDATE table_name SET column=value [WHERE condition]
  DELETE FROM table_name [WHERE condition]
  
Special Commands:
  tables    - Show all tables
  help      - Show this help
  exit/quit - Exit the program
  
Data Types:
  INTEGER
  VARCHAR(size)
  BOOLEAN
  
Constraints:
  NOT NULL
  PRIMARY KEY
  
Examples:
  CREATE TABLE users (id INTEGER PRIMARY KEY, name VARCHAR(50) NOT NULL, age INTEGER);
  INSERT INTO users VALUES (1, 'Alice', 25);
  SELECT * FROM users WHERE age > 20;
  UPDATE users SET age = 26 WHERE name = 'Alice';
  DELETE FROM users WHERE id = 1;
`)
}

// テーブル一覧表示
func showTables(db *Database) {
	if len(db.Tables) == 0 {
		fmt.Println("No tables found")
		return
	}

	fmt.Println("Tables:")
	for name, table := range db.Tables {
		fmt.Printf("  %s (", name)
		cols := []string{}
		for _, col := range table.Columns {
			colStr := fmt.Sprintf("%s %s", col.Name, col.Type)
			if col.Primary {
				colStr += " PRIMARY KEY"
			}
			if col.NotNull {
				colStr += " NOT NULL"
			}
			cols = append(cols, colStr)
		}
		fmt.Printf("%s)\n", strings.Join(cols, ", "))
	}
}
