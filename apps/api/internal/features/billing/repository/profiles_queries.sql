-- ---------- Billing profiles (personal data: never logged) ----------

-- name: GetBillingProfile :one
SELECT legal_name, email, phone, address_line1, address_line2, city, state, postal_code, country,
       tax_id_type, tax_id, updated_at
FROM billing_profiles
WHERE workspace_id = @workspace_id;

-- name: UpsertBillingProfile :one
INSERT INTO billing_profiles (workspace_id, legal_name, email, phone, address_line1, address_line2, city, state,
                              postal_code, country, tax_id_type, tax_id)
VALUES (@workspace_id, @legal_name, @email, sqlc.narg('phone'), sqlc.narg('address_line1'),
        sqlc.narg('address_line2'), sqlc.narg('city'), sqlc.narg('state'), sqlc.narg('postal_code'),
        sqlc.narg('country'), sqlc.narg('tax_id_type'), sqlc.narg('tax_id'))
ON CONFLICT (workspace_id) DO UPDATE SET
    legal_name    = EXCLUDED.legal_name,
    email         = EXCLUDED.email,
    phone         = EXCLUDED.phone,
    address_line1 = EXCLUDED.address_line1,
    address_line2 = EXCLUDED.address_line2,
    city          = EXCLUDED.city,
    state         = EXCLUDED.state,
    postal_code   = EXCLUDED.postal_code,
    country       = EXCLUDED.country,
    tax_id_type   = EXCLUDED.tax_id_type,
    tax_id        = EXCLUDED.tax_id
RETURNING legal_name, email, phone, address_line1, address_line2, city, state, postal_code, country,
          tax_id_type, tax_id, updated_at;

-- ---------- Provider customers ----------

-- name: GetBillingCustomer :one
SELECT id, provider, external_customer_id
FROM billing_customers
WHERE workspace_id = @workspace_id AND provider = @provider;

-- name: EnsureBillingCustomer :one
-- Returns the workspace's customer row for provider, creating it on first use.
INSERT INTO billing_customers (workspace_id, provider)
VALUES (@workspace_id, @provider)
ON CONFLICT (workspace_id, provider) DO UPDATE SET provider = EXCLUDED.provider
RETURNING id, provider, external_customer_id;

-- name: SetBillingCustomerExternalID :execrows
UPDATE billing_customers
SET external_customer_id = @external_customer_id
WHERE workspace_id = @workspace_id AND id = @billing_customer_id
  AND (external_customer_id IS NULL OR external_customer_id = @external_customer_id);
