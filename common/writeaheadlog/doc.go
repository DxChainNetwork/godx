package writeaheadlog

// wal is a high-performance writeaheadlog, which is used for recoverable logging.
//
// When creating wal, a file used for logging will be created specified by user-defined path. Afterwards, all uncommitted transactions are stored in the log file.
//
// A transaction is a batch of Operations, specified by the name, and the payload.
//
// A transaction have four available commands:
//  1. NewTransaction(ops []Operation) - Create a new transaction with a batch of operations
//  2. Append(ops []Operation)         - Append a batch of operations to the existing transaction
//  3. Commit()                        - Inform the transaction is committed. Only committed transactions are recovered when restart the Wal.
//  4. Release()                       - Release the transaction, and wal can use the storage of the transaction in the log file.
//
//  Specified example please read Example provided.
