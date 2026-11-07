-- +goose Up
-- Relay issues its own invoice documents (ADR 0016): its own numbers, a PDF in
-- object storage, and a "payment received" email with the PDF attached.

-- One row per calendar year (UTC, of paid_at). The paid invoice's transaction
-- bumps last_seq, so a rollback leaves no gap in the year's sequence.
CREATE TABLE invoice_number_counters (
    year     int NOT NULL PRIMARY KEY CHECK (year > 0),
    last_seq int NOT NULL CHECK (last_seq > 0)
);

ALTER TABLE invoices
    ADD COLUMN pdf_key    text,
    ADD COLUMN emailed_at timestamptz,
    DROP COLUMN receipt_url;

-- invoices.number held the provider's number. Relay numbers paid invoices
-- (and those later refunded or disputed) in payment order; the rest have none.
-- The prefix is INVOICE_NUMBER_PREFIX's default.
UPDATE invoices SET number = NULL WHERE number IS NOT NULL;

WITH numbered AS (
    SELECT id,
           extract(year FROM COALESCE(paid_at, created_at) AT TIME ZONE 'UTC')::int AS year,
           row_number() OVER (
               PARTITION BY extract(year FROM COALESCE(paid_at, created_at) AT TIME ZONE 'UTC')
               ORDER BY COALESCE(paid_at, created_at), id) AS seq
    FROM invoices
    WHERE status IN ('paid', 'partially_refunded', 'refunded', 'disputed')
)
UPDATE invoices i
SET number = 'RLY-' || n.year || '-' || lpad(n.seq::text, greatest(6, length(n.seq::text)), '0')
FROM numbered n
WHERE i.id = n.id;

INSERT INTO invoice_number_counters (year, last_seq)
SELECT split_part(number, '-', 2)::int, max(split_part(number, '-', 3)::int)
FROM invoices
WHERE number IS NOT NULL
GROUP BY 1;

ALTER TABLE invoices ADD CONSTRAINT invoices_number_key UNIQUE (number);

-- Past payments get their PDF from the hourly sweep, but no email: the
-- customer was told about them long ago.
UPDATE invoices SET emailed_at = COALESCE(paid_at, created_at) WHERE number IS NOT NULL;

-- The sweep's work list: numbered invoices still waiting for their PDF.
CREATE INDEX invoices_pdf_pending_idx ON invoices (created_at) WHERE number IS NOT NULL AND pdf_key IS NULL;

-- +goose Down
-- Provider numbers and receipt links are gone for good; Relay's numbers stay.
DROP INDEX invoices_pdf_pending_idx;
ALTER TABLE invoices DROP CONSTRAINT invoices_number_key;

ALTER TABLE invoices
    ADD COLUMN receipt_url text,
    DROP COLUMN emailed_at,
    DROP COLUMN pdf_key;

DROP TABLE invoice_number_counters;
