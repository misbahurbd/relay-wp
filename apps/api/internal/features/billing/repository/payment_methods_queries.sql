-- name: GetCardCustomer :one
-- The provider customer whose card pays next: the live subscription's, else
-- the live lifetime deal's, else the newest one the provider knows. The card
-- itself stays at the provider.
SELECT c.provider, c.external_customer_id, s.external_subscription_id
FROM billing_customers c
LEFT JOIN subscriptions s
       ON s.billing_customer_id = c.id AND s.workspace_id = c.workspace_id
      AND s.status IN ('trialing', 'active', 'past_due')
LEFT JOIN orders o
       ON o.billing_customer_id = c.id AND o.workspace_id = c.workspace_id
      AND o.status IN ('in_installments', 'paid', 'defaulted')
WHERE c.workspace_id = @workspace_id AND c.external_customer_id IS NOT NULL
ORDER BY (s.id IS NOT NULL) DESC, (o.id IS NOT NULL) DESC, c.created_at DESC
LIMIT 1;
