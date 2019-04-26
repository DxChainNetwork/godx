package writeaheadlog

// writeRecoveryState is a helper function that changes the recoveryState on disk
func (w *Wal) writeRecoveryState(state uint8) error {
	_, err := w.logFile.WriteAt([]byte{byte(state)}, int64(len(metadataHeader)+len(metadataVersion)))
	if err != nil {
		return err
	}
	return w.logFile.Sync()
}
