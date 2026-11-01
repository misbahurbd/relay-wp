package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/contacts/model"
)

// Who matches a segment can't be a sqlc query: its shape is the user's rules.
// segmentWhere compiles checked rules (model.SegmentRules.Check) to one
// condition on contacts c. Fields and operators come from the catalog below;
// every value is a $n argument, never SQL text (ADR 0014).

type sqlArgs []any

func (a *sqlArgs) add(v any) string {
	*a = append(*a, v)
	return fmt.Sprintf("$%d", len(*a))
}

const tagLink = `EXISTS (SELECT 1 FROM contact_tag_links l WHERE l.workspace_id = c.workspace_id AND l.contact_id = c.id AND l.tag_id = %s)`

func segmentWhere(rules model.SegmentRules, args *sqlArgs) (string, error) {
	if msgs := rules.Check(); len(msgs) > 0 {
		return "", fmt.Errorf("segment rules: %s %s", msgs[0].Field, msgs[0].Message)
	}
	groups := make([]string, len(rules.Groups))
	for i, g := range rules.Groups {
		conds := make([]string, len(g.Conditions))
		for j, c := range g.Conditions {
			sql, err := segmentCondition(c, args)
			if err != nil {
				return "", err
			}
			conds[j] = sql
		}
		join := " AND "
		if g.Match == "any" {
			join = " OR "
		}
		groups[i] = "(" + strings.Join(conds, join) + ")"
	}
	return "(" + strings.Join(groups, " OR ") + ")", nil
}

func segmentCondition(c model.SegmentCondition, args *sqlArgs) (string, error) {
	v := strings.TrimSpace(c.Value)
	switch c.Field + ":" + c.Op {
	case model.SegTag + ":" + model.OpIs:
		return fmt.Sprintf(tagLink, args.add(uuid.MustParse(v))), nil
	case model.SegTag + ":" + model.OpIsNot:
		return "NOT " + fmt.Sprintf(tagLink, args.add(uuid.MustParse(v))), nil
	case model.SegOptIn + ":" + model.OpIs:
		return "c.opt_in = " + args.add(v), nil
	case model.SegOptIn + ":" + model.OpIsNot:
		return "c.opt_in <> " + args.add(v), nil
	case model.SegLastMessage + ":" + model.OpWithin:
		return "c.last_message_at >= now() - make_interval(days => " + args.add(int32(c.Days())) + ")", nil //nolint:gosec // checked ≤ 3650
	case model.SegLastMessage + ":" + model.OpOver:
		return "(c.last_message_at IS NULL OR c.last_message_at < now() - make_interval(days => " + args.add(int32(c.Days())) + "))", nil //nolint:gosec // checked ≤ 3650
	case model.SegLastMessage + ":" + model.OpNever:
		return "c.last_message_at IS NULL", nil
	case model.SegAdded + ":" + model.OpWithin:
		return "c.created_at >= now() - make_interval(days => " + args.add(int32(c.Days())) + ")", nil //nolint:gosec // checked ≤ 3650
	case model.SegAdded + ":" + model.OpOver:
		return "c.created_at < now() - make_interval(days => " + args.add(int32(c.Days())) + ")", nil //nolint:gosec // checked ≤ 3650
	case model.SegSource + ":" + model.OpIs:
		return "c.source = " + args.add(v), nil
	case model.SegSource + ":" + model.OpIsNot:
		return "c.source <> " + args.add(v), nil
	case model.SegLanguage + ":" + model.OpIs:
		return "lower(c.language) = lower(" + args.add(v) + ")", nil
	case model.SegLanguage + ":" + model.OpIsNot:
		return "lower(coalesce(c.language, '')) <> lower(" + args.add(v) + ")", nil
	case model.SegCountryCode + ":" + model.OpStarts:
		return "c.phone LIKE " + args.add(v+"%"), nil
	case model.SegAttribute + ":" + model.OpIs:
		return "lower(c.attributes ->> " + args.add(strings.TrimSpace(c.Key)) + ") = lower(" + args.add(v) + ")", nil
	case model.SegAttribute + ":" + model.OpContains:
		return "c.attributes ->> " + args.add(strings.TrimSpace(c.Key)) + " ILIKE " + args.add("%"+escapeLike(v)+"%"), nil
	}
	return "", fmt.Errorf("segment rules: no SQL for %s %s", c.Field, c.Op)
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// CountSegment counts the workspace's contacts matching rules, by opt-in.
func (r Repository) CountSegment(ctx context.Context, workspaceID uuid.UUID, rules model.SegmentRules) (model.SegmentCount, error) {
	args := sqlArgs{workspaceID}
	where, err := segmentWhere(rules, &args)
	if err != nil {
		return model.SegmentCount{}, err
	}
	var c model.SegmentCount
	err = r.db.QueryRow(ctx, `SELECT count(*), count(*) FILTER (WHERE c.opt_in = 'in'),
		count(*) FILTER (WHERE c.opt_in = 'out'), count(*) FILTER (WHERE c.opt_in = 'pending')
		FROM contacts c WHERE c.workspace_id = $1 AND `+where, args...).Scan(&c.Matched, &c.OptedIn, &c.OptedOut, &c.Pending)
	if err != nil {
		return c, fmt.Errorf("count segment: %w", err)
	}
	return c, nil
}

// SegmentSample is up to limit matching contacts, most recently active first.
func (r Repository) SegmentSample(ctx context.Context, workspaceID uuid.UUID, rules model.SegmentRules, limit int) ([]model.SegmentMatch, error) {
	args := sqlArgs{workspaceID}
	where, err := segmentWhere(rules, &args)
	if err != nil {
		return nil, err
	}
	lim := args.add(int32(min(limit, 100))) //nolint:gosec // capped
	rows, err := r.db.Query(ctx, `SELECT c.id, c.name, c.phone, c.opt_in FROM contacts c
		WHERE c.workspace_id = $1 AND `+where+` ORDER BY c.last_message_at DESC NULLS LAST, c.id DESC LIMIT `+lim, args...)
	if err != nil {
		return nil, fmt.Errorf("sample segment: %w", err)
	}
	defer rows.Close()
	out := []model.SegmentMatch{}
	for rows.Next() {
		var m model.SegmentMatch
		if err := rows.Scan(&m.ID, &m.Name, &m.Phone, &m.OptIn); err != nil {
			return nil, fmt.Errorf("scan segment sample: %w", err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sample segment: %w", err)
	}
	return out, nil
}

// SegmentMembers is every contact matching rules.
func (r Repository) SegmentMembers(ctx context.Context, workspaceID uuid.UUID, rules model.SegmentRules) ([]uuid.UUID, error) {
	args := sqlArgs{workspaceID}
	where, err := segmentWhere(rules, &args)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.Query(ctx, `SELECT c.id FROM contacts c WHERE c.workspace_id = $1 AND `+where, args...)
	if err != nil {
		return nil, fmt.Errorf("segment members: %w", err)
	}
	defer rows.Close()
	out := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan segment member: %w", err)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("segment members: %w", err)
	}
	return out, nil
}
