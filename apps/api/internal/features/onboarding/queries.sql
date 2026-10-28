-- name: CreateOnboarding :exec
INSERT INTO workspace_onboarding (workspace_id, first_goal, sample_data, completed_at)
VALUES (@workspace_id, @first_goal, @sample_data, @completed_at);

-- name: GetOnboarding :one
SELECT workspace_id, first_goal, sample_data, business_done_at, whatsapp_skipped_at,
       test_message_sent_at, completed_at, reminder_count
FROM workspace_onboarding
WHERE workspace_id = @workspace_id;

-- name: MarkBusinessDone :exec
UPDATE workspace_onboarding
SET business_done_at = COALESCE(business_done_at, now())
WHERE workspace_id = @workspace_id;

-- name: SkipWhatsAppStep :exec
UPDATE workspace_onboarding
SET whatsapp_skipped_at = now(), completed_at = COALESCE(completed_at, now())
WHERE workspace_id = @workspace_id;

-- name: CompleteOnboarding :exec
UPDATE workspace_onboarding
SET completed_at = COALESCE(completed_at, now())
WHERE workspace_id = @workspace_id;

-- name: MarkTestMessageSent :exec
UPDATE workspace_onboarding
SET test_message_sent_at = COALESCE(test_message_sent_at, now())
WHERE workspace_id = @workspace_id;

-- name: BumpReminderCount :exec
UPDATE workspace_onboarding
SET reminder_count = reminder_count + 1
WHERE workspace_id = @workspace_id;

-- name: GetMemberState :one
SELECT tour_seen_at, checklist_hidden_at
FROM onboarding_member_state
WHERE workspace_id = @workspace_id AND user_id = @user_id;

-- name: UpsertMemberState :exec
-- A null flag leaves that column as it is; true stamps it once; false clears it.
INSERT INTO onboarding_member_state (workspace_id, user_id, tour_seen_at, checklist_hidden_at)
VALUES (@workspace_id, @user_id,
        CASE WHEN sqlc.narg(tour_seen)::boolean THEN now() END,
        CASE WHEN sqlc.narg(checklist_hidden)::boolean THEN now() END)
ON CONFLICT (workspace_id, user_id) DO UPDATE
SET tour_seen_at = CASE
        WHEN sqlc.narg(tour_seen)::boolean IS NULL THEN onboarding_member_state.tour_seen_at
        WHEN sqlc.narg(tour_seen)::boolean THEN COALESCE(onboarding_member_state.tour_seen_at, now())
    END,
    checklist_hidden_at = CASE
        WHEN sqlc.narg(checklist_hidden)::boolean IS NULL THEN onboarding_member_state.checklist_hidden_at
        WHEN sqlc.narg(checklist_hidden)::boolean THEN COALESCE(onboarding_member_state.checklist_hidden_at, now())
    END;
