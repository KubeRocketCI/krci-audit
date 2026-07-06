-- Reverse 000004: drop the rotation function. Partitions themselves are unaffected.
DROP FUNCTION IF EXISTS audit_rotate_partitions(INTEGER, INTEGER);
