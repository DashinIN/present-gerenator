CREATE UNIQUE INDEX IF NOT EXISTS credit_transactions_type_reference_id_unique
    ON credit_transactions (type, reference_id)
    WHERE reference_id IS NOT NULL;
