package storagehost

import (
	"errors"
	"math/big"
	"strconv"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
)

const (
	obligationUnresolved storageObligationStatus = iota // Indicatees that an unitialized value was used. Unresolved
	// 表示使用了未初始化的值。
	obligationRejected // Indicates that the obligation never got started, no revenue gained or lost.
	// 表示义务从未开始，没有收入或损失。
	obligationSucceeded // Indicates that the obligation was completed, revenues were gained.
	// 表明义务已经完成，收入已经获得。
	obligationFailed // Indicates that the obligation failed, revenues and collateral were lost.
	// 表明义务失败，收入和抵押品丢失。
)

var (
	// errDuplicateStorageObligation is returned when the storage obligation
	// database already has a storage obligation with the provided file
	// contract. This error should only happen in the event of a developer
	// mistake.
	// 当存储义务数据库已经具有提供的文件合同的存储义务时，
	// 将返回errDuplicateStorageObligation。
	// 只有在开发人员出错时才会出现此错误

	// 存储义务具有与现有存储义务冲突的文件合同
	errDuplicateStorageObligation = errors.New("storage obligation has a file contract which conflicts with an existing storage obligation")

	// errInsaneFileContractOutputCounts is returned when a file contract has
	// the wrong number of outputs for either the valid or missed payouts.
	// 当文件合同的有效或错过的付款输出数量错误时，将返回errInsaneFileContractOutputCounts

	// 文件合同的有效或错过的付款的输出数量不正确
	errInsaneFileContractOutputCounts = errors.New("file contract has incorrect number of outputs for the valid or missed payouts")

	// errInsaneFileContractRevisionOutputCounts is returned when a file
	// contract has the wrong number of outputs for either the valid or missed
	// payouts.
	// 当文件合同的有效或错过的付款输出数量错误时，将返回errInsaneFileContractRevisionOutputCounts。

	// 文件合同修订的有效或错过的支出的输出数量不正确
	errInsaneFileContractRevisionOutputCounts = errors.New("file contract revision has incorrect number of outputs for the valid or missed payouts")

	// errInsaneOriginSetFileContract is returned is the final transaction of
	// the origin transaction set of a storage obligation does not have a file
	// contract in the final transaction - there should be a file contract
	// associated with every storage obligation.
	// 返回的errInsaneOriginSetFileContract是原始事务集的最终事务，
	// 存储义务在最终事务中没有文件契约 - 应该有与每个存储义务相关联的文件契约。

	//原始交易存储义务集应在最终交易中具有一个文件合同
	errInsaneOriginSetFileContract = errors.New("origin transaction set of storage obligation should have one file contract in the final transaction")

	// errInsaneOriginSetSize is returned if the origin transaction set of a
	// storage obligation is empty - there should be a file contract associated
	// with every storage obligation.
	// 如果存储义务的原始事务集为空，则返回errInsaneOriginSetSize  - 应该存在与每个存储义务关联的文件协定。

	//原始交易存储义务集的大小为零
	errInsaneOriginSetSize = errors.New("origin transaction set of storage obligation is size zero")

	// errInsaneRevisionSetRevisionCount is returned if the final transaction
	// in the revision transaction set of a storage obligation has more or less
	// than one file contract revision.
	// 如果存储义务的修订事务集中的最终事务具有多于或少于一个文件合同修订，则返回错误信号恢复设置。

	//修订交易存储义务集应在最终交易中有一个文件合同修订
	errInsaneRevisionSetRevisionCount = errors.New("revision transaction set of storage obligation should have one file contract revision in the final transaction")

	// errInsaneStorageObligationRevision is returned if there is an attempted
	// storage obligation revision which does not have sensical inputs.
	// 如果尝试存储义务修订版没有敏感输入，则返回errInsaneStorageObligationRevision。

	//存储义务的修订没有意义
	errInsaneStorageObligationRevision = errors.New("revision to storage obligation does not make sense")

	// errInsaneStorageObligationRevisionData is returned if there is an
	// attempted storage obligation revision which does not have sensical
	// inputs.
	//如果尝试存储义务修订版没有敏感输入，则返回错误信息。
	errInsaneStorageObligationRevisionData = errors.New("revision to storage obligation has insane data")

	// errNoBuffer is returned if there is an attempted storage obligation that
	// needs to have the storage proof submitted in less than
	// revisionSubmissionBuffer blocks.
	// 如果存在尝试存储义务需要在少于revisionSubmissionBuffer块中提交存储证明，则返回errNoBuffer。

	//文件合同被拒绝，因为存储证明窗口太近了
	errNoBuffer = errors.New("file contract rejected because storage proof window is too close")

	// errNoStorageObligation is returned if the requested storage obligation
	// is not found in the database. 如果在数据库中找不到请求的存储义务，则返回errNoStorageObligation。

	//在数据库中找不到存储义务
	errNoStorageObligation = errors.New("storage obligation not found in database")

	// errObligationUnlocked is returned when a storage obligation is being
	// removed from lock, but is already unlocked. 当存储义务从锁定中删除但已解锁时，将返回errObligationUnlocked。

	//存储义务已解锁，不应解锁
	errObligationUnlocked = errors.New("storage obligation is unlocked, and should not be getting unlocked")
)

type (
	StorageObligation struct {
		// Storage obligations are broken up into ordered atomic sectors that are
		// exactly 4MiB each. By saving the roots of each sector, storage proofs
		// and modifications to the data can be made inexpensively by making use of
		// the merkletree.CachedTree. Sectors can be appended, modified, or deleted
		// and the host can recompute the Merkle root of the whole file without
		// much computational or I/O expense.
		//存储义务被分解为有序的原子扇区，每个扇区正好是4MiB。 通过保存每个扇区的根，
		// 可以通过使用merkletree.CachedTree以低成本方式进行存储证明和对数据的修改。
		// 可以附加，修改或删除扇区，主机可以重新计算整个文件的Merkle根，而无需太多的计算或I / O开销。
		SectorRoots common.Hash

		// Variables about the file contract that enforces the storage obligation.
		// The origin an revision transaction are stored as a set, where the set
		// contains potentially unconfirmed transactions.
		//关于强制执行存储义务的文件合同的变量。 原始修订事务存储为集合，其中集合包含可能未经证实的事务。
		ContractCost             *big.Int //合同花销
		LockedCollateral         *big.Int //锁定质押
		PotentialDownloadRevenue *big.Int //潜在的下载收入
		PotentialStorageRevenue  *big.Int //潜在的存储收入
		PotentialUploadRevenue   *big.Int //潜在的上传收入
		RiskedCollateral         *big.Int //风险质押
		TransactionFeesAdded     *big.Int

		// The negotiation height specifies the block height at which the file
		// contract was negotiated. If the origin transaction set is not accepted
		// onto the blockchain quickly enough, the contract is pruned from the
		// host. The origin and revision transaction set contain the contracts +
		// revisions as well as all parent transactions. The parents are necessary
		// because after a restart the transaction pool may be emptied out.
		// 协商高度指定协商文件合同的块高度。
		// 如果原始交易集未被足够快地接受到区块链上，则合同将从主机中删除。
		// 原始和修订事务集包含合同+修订以及所有父事务。 父母是必要的，因为重新启动后，事务池可能会被清空。
		NegotiationHeight      *big.Int
		OriginTransactionSet   []types.Signature
		RevisionTransactionSet []types.Signature

		// Variables indicating whether the critical transactions in a storage
		// obligation have been confirmed on the blockchain.
		// 变量，指示是否已在区块链上确认存储义务中的关键事务。
		ObligationStatus    storageObligationStatus
		OriginConfirmed     bool
		ProofConfirmed      bool
		ProofConstructed    bool
		RevisionConfirmed   bool
		RevisionConstructed bool
	}

	storageObligationStatus uint64
)

func (i storageObligationStatus) String() string {
	if i == 0 {
		return "obligationUnresolved" // 未解决
	}
	if i == 1 {
		return "obligationRejected" // 被拒绝
	}
	if i == 2 {
		return "obligationSucceeded" // 成功
	}
	if i == 3 {
		return "obligationFailed" // 失败
	}
	return "storageObligationStatus(" + strconv.FormatInt(int64(i), 10) + ")"
}
