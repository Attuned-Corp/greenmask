package subset

import (
	"fmt"
	"slices"
	"strings"

	"github.com/greenmaskio/greenmask/internal/db/postgres/entries"
)

type cteQuery struct {
	items []*cteItem
	c     *Component
	// names keeps track of already added CTE names to avoid duplicates in WITH list
	names map[string]bool
}

func newCteQuery(c *Component) *cteQuery {
	return &cteQuery{
		c:     c,
		names: make(map[string]bool),
	}
}

func (c *cteQuery) addItem(name, query string) {
	if c.names[name] {
		// Already added; skip to prevent "WITH query name ... specified more than once"
		return
	}
	c.items = append(c.items, &cteItem{name: name, query: query})
	c.names[name] = true
}

func (c *cteQuery) generateQuery(targetTable *entries.Table) string {
	var queries []string
	var excludedCteQueries []string
	if len(c.c.groupedCycles) > 1 {
		panic("FIXME: found more than one grouped cycle")
	}
	for _, edge := range c.c.cycles[0] {
		if edge.from.table.Oid == targetTable.Oid {
			continue
		}
		excludedCteQuery := fmt.Sprintf("%s__%s__ids", edge.from.table.Schema, edge.from.table.Name)
		excludedCteQueries = append(excludedCteQueries, excludedCteQuery)
	}

	for _, item := range c.items {
		if slices.Contains(excludedCteQueries, item.name) {
			continue
		}
		queries = append(queries, fmt.Sprintf(" %s AS (%s)", item.name, item.query))
	}
	var leftTableKeys, rightTableKeys []string
	rightTableName := fmt.Sprintf("%s__%s__ids", targetTable.Schema, targetTable.Name)
	for _, key := range targetTable.PrimaryKey {
		leftTableKeys = append(leftTableKeys, fmt.Sprintf(`"%s"."%s"."%s"`, targetTable.Schema, targetTable.Name, key))
		rightTableKeys = append(rightTableKeys, fmt.Sprintf(`"%s"."%s"`, rightTableName, key))
	}
	leftKeysCSV := strings.Join(leftTableKeys, ",")
	rightKeysCSV := strings.Join(rightTableKeys, ",")
	// Build explicit non-generated column list using shared helper
	selectClause := generateSelectAllColumns(targetTable)

	resultingQuery := fmt.Sprintf(
		`%s FROM "%s"."%s" WHERE (%s) IN (SELECT %s FROM "%s")`,
		selectClause,
		targetTable.Schema,
		targetTable.Name,
		leftKeysCSV,
		rightKeysCSV,
		rightTableName,
	)
	res := fmt.Sprintf("WITH RECURSIVE %s %s", strings.Join(queries, ","), resultingQuery)
	return res
}

type cteItem struct {
	name  string
	query string
}
