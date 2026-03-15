-- name: ListRadacctRowsAfterID :many
SELECT *
FROM radacct
WHERE radacctid > sqlc.arg(after_radacct_id)
ORDER BY radacctid ASC
LIMIT sqlc.arg(limit_count);
