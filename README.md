# シンプルRDBMS - Go実装

Go言語で実装した軽量なリレーショナルデータベース管理システム（RDBMS）です。基本的なSQL操作をサポートし、データをJSONファイルとして永続化します。

## 特徴

- 📝 基本的なSQL文をサポート（CREATE, INSERT, SELECT, UPDATE, DELETE）
- 💾 JSONファイルによるデータ永続化
- 🔍 WHERE句による条件検索
- 🔑 PRIMARY KEY制約
- ✅ NOT NULL制約
- 📊 3つの基本データ型（INTEGER, VARCHAR, BOOLEAN）
- 🎯 LIKE演算子によるパターンマッチング

## インストールと実行

```bash
# リポジトリのクローン
git clone <repository-url>
cd simple-rdbms

# 実行
go run rdbms.go
```

## 使い方

### 基本コマンド

起動すると対話型のSQLプロンプトが表示されます：

```
Simple RDBMS - Type 'help' for commands
========================================

SQL> 
```

### 特殊コマンド

| コマンド | 説明 |
|---------|------|
| `help` | ヘルプを表示 |
| `tables` | 全テーブルの一覧を表示 |
| `exit` / `quit` | プログラムを終了 |

## SQL構文

### CREATE TABLE

テーブルを作成します。

```sql
CREATE TABLE table_name (
    column_name data_type [constraints],
    ...
);
```

**例：**
```sql
CREATE TABLE users (
    id INTEGER PRIMARY KEY,
    name VARCHAR(50) NOT NULL,
    age INTEGER,
    active BOOLEAN
);

CREATE TABLE products (
    id INTEGER PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    price INTEGER,
    in_stock BOOLEAN
);
```

### INSERT

データを挿入します。

```sql
-- 全カラムに値を挿入
INSERT INTO table_name VALUES (value1, value2, ...);

-- 特定のカラムに値を挿入
INSERT INTO table_name (column1, column2, ...) VALUES (value1, value2, ...);
```

**例：**
```sql
INSERT INTO users VALUES (1, 'Alice', 25, TRUE);
INSERT INTO users VALUES (2, 'Bob', 30, FALSE);
INSERT INTO users (id, name, age) VALUES (3, 'Charlie', 28);
```

### SELECT

データを検索します。

```sql
-- 全カラムを取得
SELECT * FROM table_name;

-- 特定のカラムを取得
SELECT column1, column2 FROM table_name;

-- 条件付き検索
SELECT * FROM table_name WHERE condition;
```

**例：**
```sql
SELECT * FROM users;
SELECT name, age FROM users;
SELECT * FROM users WHERE age > 25;
SELECT * FROM users WHERE name = 'Alice';
SELECT * FROM users WHERE active = TRUE;
SELECT * FROM users WHERE name LIKE 'A%';
```

### UPDATE

データを更新します。

```sql
UPDATE table_name SET column1 = value1, column2 = value2 WHERE condition;
```

**例：**
```sql
UPDATE users SET age = 26 WHERE name = 'Alice';
UPDATE users SET active = TRUE WHERE age >= 25;
UPDATE users SET name = 'Robert', age = 31 WHERE name = 'Bob';
```

### DELETE

データを削除します。

```sql
DELETE FROM table_name WHERE condition;

-- 全行削除（注意！）
DELETE FROM table_name;
```

**例：**
```sql
DELETE FROM users WHERE id = 1;
DELETE FROM users WHERE active = FALSE;
DELETE FROM users WHERE age < 25;
```

## データ型

| データ型 | 説明 | 例 |
|---------|------|-----|
| `INTEGER` | 整数 | 1, -100, 0 |
| `VARCHAR(n)` | 最大n文字の文字列 | 'Hello', 'World' |
| `BOOLEAN` | 真偽値 | TRUE, FALSE |

## 制約

| 制約 | 説明 |
|------|------|
| `PRIMARY KEY` | 主キー（一意で非NULL） |
| `NOT NULL` | NULL値を許可しない |

## WHERE句の演算子

| 演算子 | 説明 | 例 |
|--------|------|-----|
| `=` | 等しい | `WHERE age = 25` |
| `!=`, `<>` | 等しくない | `WHERE age != 25` |
| `>` | より大きい | `WHERE age > 25` |
| `>=` | 以上 | `WHERE age >= 25` |
| `<` | より小さい | `WHERE age < 25` |
| `<=` | 以下 | `WHERE age <= 25` |
| `LIKE` | パターンマッチ | `WHERE name LIKE 'A%'` |
| `IS` | NULL判定 | `WHERE age IS NULL` |
| `IS NOT` | 非NULL判定 | `WHERE age IS NOT NULL` |

### LIKEパターン

- `%` - 0文字以上の任意の文字列
- `_` - 1文字の任意の文字

**例：**
- `'A%'` - 'A'で始まる文字列
- `'%son'` - 'son'で終わる文字列
- `'%ob%'` - 'ob'を含む文字列
- `'_ob'` - 3文字で'ob'で終わる文字列

## データの保存場所

データは`./db_mydb/`ディレクトリに保存されます：

```
db_mydb/
├── metadata.json    # テーブル定義
├── users.json       # usersテーブルのデータ
└── products.json    # productsテーブルのデータ
```

## 実装の特徴

### アーキテクチャ

- **Database**: データベース全体を管理
- **Table**: テーブル構造とデータを保持
- **Column**: カラム定義（名前、型、制約）
- **Row**: 行データ（map[string]interface{}）
- **SQLParser**: SQL文を解析して実行

### エラーハンドリング

- 存在しないテーブルへのアクセス
- 重複する主キー
- NOT NULL制約違反
- データ型の不一致
- 無効なSQL構文

## 制限事項

現在の実装では以下の機能は**サポートされていません**：

- 複数のWHERE条件（AND/OR）
- JOIN操作
- GROUP BY / ORDER BY
- 集約関数（COUNT, SUM, AVG等）
- インデックス
- トランザクション
- 外部キー制約
- デフォルト値
- AUTO_INCREMENT

## 今後の拡張案

### 1. インデックス実装
```go
// B-Treeインデックスの基本構造
type Index struct {
    Name    string
    Table   string
    Column  string
    Type    string      // "BTREE" or "HASH"
    Root    *BTreeNode
}

type BTreeNode struct {
    Keys     []interface{}
    Values   [][]int      // 行インデックスの配列
    Children []*BTreeNode
    IsLeaf   bool
}
```

### 2. JOIN操作
```sql
-- 将来的な実装例
SELECT u.name, o.total 
FROM users u 
JOIN orders o ON u.id = o.user_id;
```

### 3. トランザクション
```go
// トランザクション管理の基本構造
type Transaction struct {
    ID        string
    StartTime time.Time
    Status    string // "ACTIVE", "COMMITTED", "ABORTED"
    Logs      []TransactionLog
    Locks     []Lock
}
```

### 4. 集約関数
```sql
-- 将来的な実装例
SELECT COUNT(*) FROM users;
SELECT AVG(age) FROM users;
SELECT SUM(price) FROM products WHERE in_stock = TRUE;
```

## サンプルセッション

```sql
SQL> CREATE TABLE employees (id INTEGER PRIMARY KEY, name VARCHAR(100) NOT NULL, department VARCHAR(50), salary INTEGER);
Table 'employees' created successfully

SQL> INSERT INTO employees VALUES (1, 'John Doe', 'Engineering', 75000);
1 row inserted

SQL> INSERT INTO employees VALUES (2, 'Jane Smith', 'Marketing', 65000);
1 row inserted

SQL> INSERT INTO employees VALUES (3, 'Bob Johnson', 'Engineering', 80000);
1 row inserted

SQL> SELECT * FROM employees WHERE department = 'Engineering';
--------------------------------------------------------------------------------
| id                   | name                 | department           | salary               |
--------------------------------------------------------------------------------
| 1                    | John Doe             | Engineering          | 75000                |
| 3                    | Bob Johnson          | Engineering          | 80000                |
--------------------------------------------------------------------------------
2 row(s) returned

SQL> UPDATE employees SET salary = 77000 WHERE id = 1;
1 row(s) updated

SQL> SELECT name, salary FROM employees WHERE salary > 70000;
--------------------------------------------------------------------------------
| name                 | salary               |
--------------------------------------------------------------------------------
| John Doe             | 77000                |
| Bob Johnson          | 80000                |
--------------------------------------------------------------------------------
2 row(s) returned

SQL> DELETE FROM employees WHERE department = 'Marketing';
1 row(s) deleted

SQL> tables
Tables:
  employees (id INTEGER PRIMARY KEY, name VARCHAR NOT NULL, department VARCHAR, salary INTEGER)
```

## ライセンス

このプロジェクトは学習目的で作成されています。

## 貢献

バグ報告や機能提案は、Issueを作成してください。
