package subset

import (
	"crypto/sha1"
	"fmt"
	"sort"
	"strings"

	"github.com/greenmaskio/greenmask/internal/db/postgres/entries"
	"github.com/greenmaskio/greenmask/pkg/toolkit"
)

const (
	joinTypeInner = "INNER"
	joinTypeLeft  = "LEFT"
)

func generateJoinClauseForDroppedEdge(edge *Edge, initTableName string) string {
	var conds []string

	var leftTableKeys []string
	table := edge.from.table
	for _, key := range edge.from.keys {
		leftTableKeys = append(leftTableKeys, shorten(fmt.Sprintf(`%s__%s__%s`, table.Schema, table.Name, key.Name)))
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
	// Precompute override/alias once
	override, overridden := overriddenTables[rightTable.Table.Oid]
	alias := ""
	if overridden {
		alias = fmt.Sprintf("%s_e%d", override, edge.id)
	}
	for idx := 0; idx < len(edge.from.keys); idx++ {
		leftPart := edge.from.keys[idx].GetKeyReference(leftTable)
		rightPart := edge.to.keys[idx].GetKeyReference(rightTable)
		if overridden {
			rightPart = fmt.Sprintf(`"%s"."%s"`, alias, edge.to.keys[idx].Name)
		}
		conds = append(conds, fmt.Sprintf(`%s = %s`, leftPart, rightPart))
	}
	// If the right table is overridden by a CTE, its subset conditions are already applied inside that CTE.
	if !overridden && len(edge.to.table.SubsetConds) > 0 {
		conds = append(conds, edge.to.table.SubsetConds...)
	}

	if len(edge.from.polymorphicExprs) > 0 {
		conds = append(conds, edge.from.polymorphicExprs...)
	}
	if len(edge.to.polymorphicExprs) > 0 {
		conds = append(conds, edge.to.polymorphicExprs...)
	}

	rightTableName := fmt.Sprintf(`"%s"."%s"`, rightTable.Table.Schema, rightTable.Table.Name)
	if overridden {
		rightTableName = fmt.Sprintf(`%s AS %s`, override, alias)
	}

	joinClause := fmt.Sprintf(
		`%s JOIN %s ON %s`,
		joinType,
		rightTableName,
		strings.Join(conds, " AND "),
	)
	return joinClause
}

// coalesceAlias removed; aliasing is handled inside generateJoinClauseV2

// shorten returns a postgres-safe identifier not exceeding 63 bytes by hashing long names.
func shorten(name string) string {
	const maxLen = 63 // Postgres limit for identifiers (NAMEDATALEN-1)
	if len(name) <= maxLen {
		return name
	}
	h := sha1.Sum([]byte(name))
	// keep a small readable prefix and append 10 hex chars of the hash
	prefixLen := 40
	if prefixLen > maxLen-11 { // underscore + 10 hex
		prefixLen = maxLen - 11
	}
	return fmt.Sprintf("%s_%x", name[:prefixLen], h[:5])
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

// generateSelectAllColumns builds an explicit select list for non-generated columns
// to keep the column count/order consistent with table.Columns used by COPY pipeline.
func generateSelectAllColumns(table *entries.Table) string {
	var cols []string
	for _, c := range table.Columns {
		if c.IsGenerated {
			continue
		}
		cols = append(cols, fmt.Sprintf(`"%s"."%s"."%s"`, table.Schema, table.Name, c.Name))
	}
	return fmt.Sprintf(`SELECT %s`, strings.Join(cols, ", "))
}

// dedupeStrings returns a new slice with duplicate strings removed, preserving first-seen order.
func dedupeStrings(values []string) []string {
	if len(values) <= 1 {
		return values
	}
	seen := make(map[string]struct{}, len(values))
	var out []string
	for _, v := range values {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

// validExprOrTrue returns a SQL fragment "(<exprs AND-joined>) AS valid" or "TRUE AS valid" when empty.
func validExprOrTrue(exprs []string) string {
	joined := strings.TrimSpace(strings.Join(exprs, " AND "))
	if joined == "" {
		return "TRUE AS valid"
	}
	return fmt.Sprintf("(%s) AS valid", joined)
}

// buildWithClause assembles a deterministic WITH clause that orders CTEs by dependency when they reference each other.
// Returns a string like: "WITH name1 AS (...), name2 AS (...)".
func buildWithClause(cteDefs map[string]string) string {
	if len(cteDefs) == 0 {
		return ""
	}
	var names []string
	for name := range cteDefs {
		names = append(names, name)
	}
	sort.Strings(names)
	dependsOn := make(map[string]map[string]struct{}, len(names))
	inDegree := make(map[string]int, len(names))
	for _, n := range names {
		dependsOn[n] = make(map[string]struct{})
	}
	for _, n := range names {
		body := cteDefs[n]
		for _, m := range names {
			if n == m {
				continue
			}
			needle := fmt.Sprintf(`"%s"`, m)
			if strings.Contains(body, needle) {
				if _, ok := dependsOn[n][m]; !ok {
					dependsOn[n][m] = struct{}{}
					inDegree[n]++
				}
			}
		}
	}
	var ordered []string
	var zero []string
	for _, n := range names {
		if inDegree[n] == 0 {
			zero = append(zero, n)
		}
	}
	sort.Strings(zero)
	for len(zero) > 0 {
		n := zero[0]
		zero = zero[1:]
		ordered = append(ordered, n)
		for m := range dependsOn {
			if _, ok := dependsOn[m][n]; ok {
				delete(dependsOn[m], n)
				inDegree[m]--
				if inDegree[m] == 0 {
					zero = append(zero, m)
					sort.Strings(zero)
				}
			}
		}
	}
	if len(ordered) != len(names) {
		ordered = names
	}
	var parts []string
	for _, name := range ordered {
		body := cteDefs[name]
		parts = append(parts, fmt.Sprintf(`"%s" AS (%s)`, name, body))
	}
	return fmt.Sprintf("WITH %s", strings.Join(parts, ", "))
}
