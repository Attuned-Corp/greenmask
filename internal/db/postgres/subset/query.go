package subset

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/greenmaskio/greenmask/internal/db/postgres/entries"
	"github.com/greenmaskio/greenmask/pkg/toolkit"
)

const pgMaxIdentLen = 63

// truncateIdent truncates an identifier to fit PostgreSQL's 63-character limit.
// When truncation is needed, the last 8 characters are replaced with a short
// hash of the full identifier to avoid collisions.
func truncateIdent(ident string) string {
	if len(ident) <= pgMaxIdentLen {
		return ident
	}
	hash := md5.Sum([]byte(ident))
	suffix := hex.EncodeToString(hash[:])[:8]
	return ident[:pgMaxIdentLen-9] + "_" + suffix
}

const (
	joinTypeInner = "INNER"
	joinTypeLeft  = "LEFT"
)

func generateJoinClauseForDroppedEdge(edge *Edge, initTableName string) string {
	var conds []string

	var leftTableKeys []string
	table := edge.from.table
	for _, key := range edge.from.keys {
		leftTableKeys = append(leftTableKeys, columnAlias(table.Schema, table.Name, key.Name))
	}

	rightTable := edge.to
	for idx := 0; idx < len(edge.to.keys); idx++ {

		leftPart := fmt.Sprintf(
			`"%s"."%s"`,
			initTableName,
			leftTableKeys[idx],
		)

		rightPart := edge.to.keys[idx].GetKeyReference(rightTable.table)
		conds = append(conds, fmt.Sprintf(`%s = %s`, leftPart, rightPart))
	}
	if len(edge.from.polymorphicExprs) > 0 {
		conds = append(conds, edge.from.polymorphicExprs...)
	}
	if len(edge.to.polymorphicExprs) > 0 {
		conds = append(conds, edge.to.polymorphicExprs...)
	}

	rightTableName := fmt.Sprintf(`"%s"."%s"`, edge.to.table.Schema, edge.to.table.Name)

	joinClause := fmt.Sprintf(
		`JOIN %s ON %s`,
		rightTableName,
		strings.Join(conds, " AND "),
	)
	return joinClause
}

func generateJoinClauseV2(edge *Edge, joinType string, overriddenTables map[toolkit.Oid]string) string {
	if joinType != joinTypeInner && joinType != joinTypeLeft {
		panic(fmt.Sprintf("invalid join type: %s", joinType))
	}

	var conds []string

	leftTable, rightTable := edge.from.table, edge.to.table
	for idx := 0; idx < len(edge.from.keys); idx++ {

		leftPart := edge.from.keys[idx].GetKeyReference(leftTable)
		rightPart := edge.to.keys[idx].GetKeyReference(rightTable)

		if override, ok := overriddenTables[rightTable.Oid]; ok {
			rightPart = fmt.Sprintf(
				`"%s"."%s"`,
				override,
				edge.to.keys[idx].Name,
			)
		}

		conds = append(conds, fmt.Sprintf(`%s = %s`, leftPart, rightPart))
	}

	// Only add SubsetConds when the table is not overridden by a CTE.
	// When overridden, the CTE already contains filtered data and the
	// SubsetConds reference the original table name which is not in scope.
	if _, overridden := overriddenTables[rightTable.Oid]; !overridden && len(edge.to.table.SubsetConds) > 0 {
		conds = append(conds, edge.to.table.SubsetConds...)
	}

	if len(edge.from.polymorphicExprs) > 0 {
		conds = append(conds, edge.from.polymorphicExprs...)
	}
	if len(edge.to.polymorphicExprs) > 0 {
		conds = append(conds, edge.to.polymorphicExprs...)
	}

	rightTableName := fmt.Sprintf(`"%s"."%s"`, rightTable.Schema, rightTable.Name)
	if override, ok := overriddenTables[rightTable.Oid]; ok {
		// When joining to a CTE multiple times in the same FROM clause,
		// PostgreSQL requires unique aliases. Use the from table name to
		// create a distinct alias for each JOIN occurrence.
		alias := fmt.Sprintf("%s__%s", override, leftTable.Name)
		rightTableName = fmt.Sprintf(`%s AS %s`, override, alias)
		// Rewrite the right-side key references to use the alias
		for idx := range conds {
			conds[idx] = strings.ReplaceAll(conds[idx], fmt.Sprintf(`"%s"`, override), fmt.Sprintf(`"%s"`, alias))
		}
	}

	joinClause := fmt.Sprintf(
		`%s JOIN %s ON %s`,
		joinType,
		rightTableName,
		strings.Join(conds, " AND "),
	)
	return joinClause
}

func generateWhereClause(subsetConds []string) string {
	if len(subsetConds) == 0 {
		return "WHERE TRUE"
	}
	escapedConds := make([]string, 0, len(subsetConds))
	for _, cond := range subsetConds {
		escapedConds = append(escapedConds, fmt.Sprintf(`( %s )`, cond))
	}
	return "WHERE " + strings.Join(escapedConds, " AND ")
}

func generateIntegrityCheckExpr(checks []string) string {
	if len(checks) == 0 {
		return "TRUE AS valid"
	}
	return fmt.Sprintf("(%s) AS valid", strings.Join(checks, " AND "))
}

// columnAlias generates a truncation-safe alias for a column in the form schema__table__column.
func columnAlias(schema, table, column string) string {
	return truncateIdent(fmt.Sprintf("%s__%s__%s", schema, table, column))
}

// columnPathAlias generates a truncation-safe alias for a path array column.
func columnPathAlias(schema, table, column string) string {
	return truncateIdent(fmt.Sprintf("%s__%s__%s__path", schema, table, column))
}

func generateSelectByPrimaryKey(table *entries.Table, pk []string) string {
	var keys []string
	for _, key := range pk {
		keys = append(keys, fmt.Sprintf(`"%s"."%s"."%s"`, table.Schema, table.Name, key))
	}
	return fmt.Sprintf(
		`SELECT %s`,
		strings.Join(keys, ", "),
	)
}
