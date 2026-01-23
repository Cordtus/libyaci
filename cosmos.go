package libyaci

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Cosmos SDK gRPC method paths
const (
	// Tendermint/CometBFT service
	methodGetLatestBlock        = "cosmos.base.tendermint.v1beta1.Service.GetLatestBlock"
	methodGetBlockByHeight      = "cosmos.base.tendermint.v1beta1.Service.GetBlockByHeight"
	methodGetNodeInfo           = "cosmos.base.tendermint.v1beta1.Service.GetNodeInfo"
	methodGetSyncing            = "cosmos.base.tendermint.v1beta1.Service.GetSyncing"
	methodGetLatestValidatorSet = "cosmos.base.tendermint.v1beta1.Service.GetLatestValidatorSet"
	methodGetValidatorSetByH    = "cosmos.base.tendermint.v1beta1.Service.GetValidatorSetByHeight"

	// Transaction service
	methodGetTxsEvent     = "cosmos.tx.v1beta1.Service.GetTxsEvent"
	methodGetTx           = "cosmos.tx.v1beta1.Service.GetTx"
	methodGetBlockWithTxs = "cosmos.tx.v1beta1.Service.GetBlockWithTxs"

	// Auth queries
	methodAccount          = "cosmos.auth.v1beta1.Query.Account"
	methodAccounts         = "cosmos.auth.v1beta1.Query.Accounts"
	methodAccountInfo      = "cosmos.auth.v1beta1.Query.AccountInfo"
	methodModuleAccounts   = "cosmos.auth.v1beta1.Query.ModuleAccounts"
	methodModuleAccountByN = "cosmos.auth.v1beta1.Query.ModuleAccountByName"
	methodBech32Prefix     = "cosmos.auth.v1beta1.Query.Bech32Prefix"
	methodAuthParams       = "cosmos.auth.v1beta1.Query.Params"

	// Authz queries
	methodGrants        = "cosmos.authz.v1beta1.Query.Grants"
	methodGranterGrants = "cosmos.authz.v1beta1.Query.GranterGrants"
	methodGranteeGrants = "cosmos.authz.v1beta1.Query.GranteeGrants"

	// Bank queries
	methodBalance          = "cosmos.bank.v1beta1.Query.Balance"
	methodAllBalances      = "cosmos.bank.v1beta1.Query.AllBalances"
	methodSpendableBalance = "cosmos.bank.v1beta1.Query.SpendableBalances"
	methodTotalSupply      = "cosmos.bank.v1beta1.Query.TotalSupply"
	methodSupplyOf         = "cosmos.bank.v1beta1.Query.SupplyOf"
	methodDenomMetadata    = "cosmos.bank.v1beta1.Query.DenomMetadata"
	methodDenomsMetadata   = "cosmos.bank.v1beta1.Query.DenomsMetadata"
	methodDenomOwners      = "cosmos.bank.v1beta1.Query.DenomOwners"
	methodBankParams       = "cosmos.bank.v1beta1.Query.Params"

	// Staking queries
	methodValidators           = "cosmos.staking.v1beta1.Query.Validators"
	methodValidator            = "cosmos.staking.v1beta1.Query.Validator"
	methodStakingParams        = "cosmos.staking.v1beta1.Query.Params"
	methodStakingPool          = "cosmos.staking.v1beta1.Query.Pool"
	methodDelegation           = "cosmos.staking.v1beta1.Query.Delegation"
	methodDelegatorDelegations = "cosmos.staking.v1beta1.Query.DelegatorDelegations"
	methodDelegatorUnbonding   = "cosmos.staking.v1beta1.Query.DelegatorUnbondingDelegations"
	methodDelegatorValidators  = "cosmos.staking.v1beta1.Query.DelegatorValidators"
	methodValidatorDelegations = "cosmos.staking.v1beta1.Query.ValidatorDelegations"
	methodUnbondingDelegation  = "cosmos.staking.v1beta1.Query.UnbondingDelegation"
	methodRedelegations = "cosmos.staking.v1beta1.Query.Redelegations"

	// Distribution queries
	methodCommunityPool          = "cosmos.distribution.v1beta1.Query.CommunityPool"
	methodDelegationRewards      = "cosmos.distribution.v1beta1.Query.DelegationRewards"
	methodDelegationTotalRewards = "cosmos.distribution.v1beta1.Query.DelegationTotalRewards"
	methodDelegatorWithdrawAddr  = "cosmos.distribution.v1beta1.Query.DelegatorWithdrawAddress"
	methodValidatorCommission    = "cosmos.distribution.v1beta1.Query.ValidatorCommission"
	methodValidatorOutstanding = "cosmos.distribution.v1beta1.Query.ValidatorOutstandingRewards"
	methodDistributionParams   = "cosmos.distribution.v1beta1.Query.Params"

	// Gov queries (v1)
	methodProposals   = "cosmos.gov.v1.Query.Proposals"
	methodProposal    = "cosmos.gov.v1.Query.Proposal"
	methodProposalV   = "cosmos.gov.v1.Query.Votes"
	methodProposalD   = "cosmos.gov.v1.Query.Deposits"
	methodTallyResult = "cosmos.gov.v1.Query.TallyResult"
	methodGovParams   = "cosmos.gov.v1.Query.Params"

	// Mint queries
	methodInflation        = "cosmos.mint.v1beta1.Query.Inflation"
	methodAnnualProvisions = "cosmos.mint.v1beta1.Query.AnnualProvisions"
	methodMintParams       = "cosmos.mint.v1beta1.Query.Params"

	// Slashing queries
	methodSigningInfo   = "cosmos.slashing.v1beta1.Query.SigningInfo"
	methodSigningInfos  = "cosmos.slashing.v1beta1.Query.SigningInfos"
	methodSlashingParam = "cosmos.slashing.v1beta1.Query.Params"

	// Evidence queries
	methodEvidence    = "cosmos.evidence.v1beta1.Query.Evidence"
	methodAllEvidence = "cosmos.evidence.v1beta1.Query.AllEvidence"

	// Feegrant queries
	methodAllowance          = "cosmos.feegrant.v1beta1.Query.Allowance"
	methodAllowances         = "cosmos.feegrant.v1beta1.Query.Allowances"
	methodAllowancesByGrantr = "cosmos.feegrant.v1beta1.Query.AllowancesByGranter"

	// Upgrade queries
	methodCurrentPlan    = "cosmos.upgrade.v1beta1.Query.CurrentPlan"
	methodAppliedPlan    = "cosmos.upgrade.v1beta1.Query.AppliedPlan"
	methodModuleVersions = "cosmos.upgrade.v1beta1.Query.ModuleVersions"

	// IBC Core - Client
	methodClientStates = "ibc.core.client.v1.Query.ClientStates"
	methodClientState  = "ibc.core.client.v1.Query.ClientState"

	// IBC Core - Connection
	methodConnections       = "ibc.core.connection.v1.Query.Connections"
	methodConnection        = "ibc.core.connection.v1.Query.Connection"
	methodClientConnections = "ibc.core.connection.v1.Query.ClientConnections"

	// IBC Core - Channel
	methodChannels           = "ibc.core.channel.v1.Query.Channels"
	methodChannel            = "ibc.core.channel.v1.Query.Channel"
	methodConnectionChannels = "ibc.core.channel.v1.Query.ConnectionChannels"

	// IBC Applications - Transfer
	methodDenomTrace        = "ibc.applications.transfer.v1.Query.Denom"
	methodDenomTraces       = "ibc.applications.transfer.v1.Query.Denoms"
	methodDenomHash         = "ibc.applications.transfer.v1.Query.DenomHash"
	methodEscrowAddress     = "ibc.applications.transfer.v1.Query.EscrowAddress"
	methodTotalEscrowDenom  = "ibc.applications.transfer.v1.Query.TotalEscrowForDenom"
	methodIBCTransferParams = "ibc.applications.transfer.v1.Query.Params"
)

// BlockResponse represents the structure returned by GetLatestBlock and GetBlockByHeight.
type BlockResponse struct {
	Block struct {
		Header struct {
			Height  string `json:"height"`
			Time    string `json:"time"`
			ChainID string `json:"chainId"`
		} `json:"header"`
		Data struct {
			Txs []string `json:"txs"`
		} `json:"data"`
	} `json:"block"`
	BlockID struct {
		Hash string `json:"hash"`
	} `json:"blockId"`
}

// TxsEventResponse represents the response from GetTxsEvent.
type TxsEventResponse struct {
	Txs         []json.RawMessage `json:"txs"`
	TxResponses []json.RawMessage `json:"txResponses"`
	Pagination  *PaginationResp   `json:"pagination,omitempty"`
	Total       string            `json:"total,omitempty"`
}

// PaginationResp represents pagination info in responses.
type PaginationResp struct {
	NextKey string `json:"nextKey,omitempty"`
	Total   string `json:"total,omitempty"`
}

// NodeInfoResponse represents the response from GetNodeInfo.
type NodeInfoResponse struct {
	DefaultNodeInfo struct {
		Network string `json:"network"`
		Moniker string `json:"moniker"`
	} `json:"defaultNodeInfo"`
	ApplicationVersion struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"applicationVersion"`
}

// BalanceResponse represents a single coin balance.
type BalanceResponse struct {
	Balance struct {
		Denom  string `json:"denom"`
		Amount string `json:"amount"`
	} `json:"balance"`
}

// AllBalancesResponse represents multiple coin balances.
type AllBalancesResponse struct {
	Balances   []Coin          `json:"balances"`
	Pagination *PaginationResp `json:"pagination,omitempty"`
}

// Coin represents a Cosmos SDK coin.
type Coin struct {
	Denom  string `json:"denom"`
	Amount string `json:"amount"`
}

// ValidatorsResponse represents the validators query response.
type ValidatorsResponse struct {
	Validators []struct {
		OperatorAddress string `json:"operatorAddress"`
		Description     struct {
			Moniker string `json:"moniker"`
		} `json:"description"`
		Status string `json:"status"`
		Tokens string `json:"tokens"`
	} `json:"validators"`
	Pagination *PaginationResp `json:"pagination,omitempty"`
}

// ModuleAccountsResponse represents the module accounts query response.
type ModuleAccountsResponse struct {
	Accounts []struct {
		Type        string `json:"@type"`
		BaseAccount struct {
			Address string `json:"address"`
		} `json:"baseAccount"`
		Name        string   `json:"name"`
		Permissions []string `json:"permissions"`
	} `json:"accounts"`
}

// StakingParamsResponse represents the staking params query response.
type StakingParamsResponse struct {
	Params struct {
		BondDenom         string `json:"bondDenom"`
		UnbondingTime     string `json:"unbondingTime"`
		MaxValidators     uint32 `json:"maxValidators"`
		MaxEntries        uint32 `json:"maxEntries"`
		HistoricalEntries uint32 `json:"historicalEntries"`
		MinCommissionRate string `json:"minCommissionRate"`
	} `json:"params"`
}

// DelegationsResponse represents delegator delegations response.
type DelegationsResponse struct {
	DelegationResponses []struct {
		Delegation struct {
			DelegatorAddress string `json:"delegatorAddress"`
			ValidatorAddress string `json:"validatorAddress"`
			Shares           string `json:"shares"`
		} `json:"delegation"`
		Balance Coin `json:"balance"`
	} `json:"delegationResponses"`
	Pagination *PaginationResp `json:"pagination,omitempty"`
}

// SyncingResponse represents the syncing status response.
type SyncingResponse struct {
	Syncing bool `json:"syncing"`
}

// ValidatorSetResponse represents the validator set response.
type ValidatorSetResponse struct {
	BlockHeight string `json:"blockHeight"`
	Validators  []struct {
		Address          string `json:"address"`
		PubKey           any    `json:"pubKey"`
		VotingPower      string `json:"votingPower"`
		ProposerPriority string `json:"proposerPriority"`
	} `json:"validators"`
	Pagination *PaginationResp `json:"pagination,omitempty"`
}

// AccountResponse represents an account query response.
type AccountResponse struct {
	Account json.RawMessage `json:"account"`
}

// AccountsResponse represents the accounts query response.
type AccountsResponse struct {
	Accounts   []json.RawMessage `json:"accounts"`
	Pagination *PaginationResp   `json:"pagination,omitempty"`
}

// AccountInfoResponse represents account info response.
type AccountInfoResponse struct {
	Info struct {
		Address       string `json:"address"`
		PubKey        any    `json:"pubKey"`
		AccountNumber string `json:"accountNumber"`
		Sequence      string `json:"sequence"`
	} `json:"info"`
}

// Bech32PrefixResponse represents bech32 prefix response.
type Bech32PrefixResponse struct {
	Bech32Prefix string `json:"bech32Prefix"`
}

// AuthParamsResponse represents auth params response.
type AuthParamsResponse struct {
	Params struct {
		MaxMemoCharacters      string `json:"maxMemoCharacters"`
		TxSigLimit             string `json:"txSigLimit"`
		TxSizeCostPerByte      string `json:"txSizeCostPerByte"`
		SigVerifyCostEd25519   string `json:"sigVerifyCostEd25519"`
		SigVerifyCostSecp256k1 string `json:"sigVerifyCostSecp256k1"`
	} `json:"params"`
}

// GrantsResponse represents authz grants response.
type GrantsResponse struct {
	Grants []struct {
		Authorization json.RawMessage `json:"authorization"`
		Expiration    string          `json:"expiration,omitempty"`
	} `json:"grants"`
	Pagination *PaginationResp `json:"pagination,omitempty"`
}

// TotalSupplyResponse represents total supply response.
type TotalSupplyResponse struct {
	Supply     []Coin          `json:"supply"`
	Pagination *PaginationResp `json:"pagination,omitempty"`
}

// SupplyOfResponse represents supply of a single denom response.
type SupplyOfResponse struct {
	Amount Coin `json:"amount"`
}

// SpendableBalancesResponse represents spendable balances response.
type SpendableBalancesResponse struct {
	Balances   []Coin          `json:"balances"`
	Pagination *PaginationResp `json:"pagination,omitempty"`
}

// DenomMetadataResponse represents denom metadata response.
type DenomMetadataResponse struct {
	Metadata struct {
		Description string `json:"description"`
		Base        string `json:"base"`
		Display     string `json:"display"`
		Name        string `json:"name"`
		Symbol      string `json:"symbol"`
		DenomUnits  []struct {
			Denom    string   `json:"denom"`
			Exponent uint32   `json:"exponent"`
			Aliases  []string `json:"aliases"`
		} `json:"denomUnits"`
	} `json:"metadata"`
}

// DenomsMetadataResponse represents all denoms metadata response.
type DenomsMetadataResponse struct {
	Metadatas  []json.RawMessage `json:"metadatas"`
	Pagination *PaginationResp   `json:"pagination,omitempty"`
}

// DenomOwnersResponse represents denom owners response.
type DenomOwnersResponse struct {
	DenomOwners []struct {
		Address string `json:"address"`
		Balance Coin   `json:"balance"`
	} `json:"denomOwners"`
	Pagination *PaginationResp `json:"pagination,omitempty"`
}

// BankParamsResponse represents bank params response.
type BankParamsResponse struct {
	Params struct {
		SendEnabled        []any `json:"sendEnabled"`
		DefaultSendEnabled bool  `json:"defaultSendEnabled"`
	} `json:"params"`
}

// ValidatorResponse represents a single validator response.
type ValidatorResponse struct {
	Validator struct {
		OperatorAddress string `json:"operatorAddress"`
		ConsensusPubkey any    `json:"consensusPubkey"`
		Jailed          bool   `json:"jailed"`
		Status          string `json:"status"`
		Tokens          string `json:"tokens"`
		DelegatorShares string `json:"delegatorShares"`
		Description     struct {
			Moniker         string `json:"moniker"`
			Identity        string `json:"identity"`
			Website         string `json:"website"`
			SecurityContact string `json:"securityContact"`
			Details         string `json:"details"`
		} `json:"description"`
		UnbondingHeight string `json:"unbondingHeight"`
		UnbondingTime   string `json:"unbondingTime"`
		Commission      struct {
			CommissionRates struct {
				Rate          string `json:"rate"`
				MaxRate       string `json:"maxRate"`
				MaxChangeRate string `json:"maxChangeRate"`
			} `json:"commissionRates"`
			UpdateTime string `json:"updateTime"`
		} `json:"commission"`
		MinSelfDelegation string `json:"minSelfDelegation"`
	} `json:"validator"`
}

// StakingPoolResponse represents staking pool response.
type StakingPoolResponse struct {
	Pool struct {
		NotBondedTokens string `json:"notBondedTokens"`
		BondedTokens    string `json:"bondedTokens"`
	} `json:"pool"`
}

// DelegationResponse represents a single delegation response.
type DelegationResponse struct {
	DelegationResponse struct {
		Delegation struct {
			DelegatorAddress string `json:"delegatorAddress"`
			ValidatorAddress string `json:"validatorAddress"`
			Shares           string `json:"shares"`
		} `json:"delegation"`
		Balance Coin `json:"balance"`
	} `json:"delegationResponse"`
}

// UnbondingDelegationResponse represents unbonding delegation response.
type UnbondingDelegationResponse struct {
	Unbond struct {
		DelegatorAddress string `json:"delegatorAddress"`
		ValidatorAddress string `json:"validatorAddress"`
		Entries          []struct {
			CreationHeight string `json:"creationHeight"`
			CompletionTime string `json:"completionTime"`
			InitialBalance string `json:"initialBalance"`
			Balance        string `json:"balance"`
		} `json:"entries"`
	} `json:"unbond"`
}

// UnbondingDelegationsResponse represents multiple unbonding delegations.
type UnbondingDelegationsResponse struct {
	UnbondingResponses []struct {
		DelegatorAddress string `json:"delegatorAddress"`
		ValidatorAddress string `json:"validatorAddress"`
		Entries          []struct {
			CreationHeight string `json:"creationHeight"`
			CompletionTime string `json:"completionTime"`
			InitialBalance string `json:"initialBalance"`
			Balance        string `json:"balance"`
		} `json:"entries"`
	} `json:"unbondingResponses"`
	Pagination *PaginationResp `json:"pagination,omitempty"`
}

// RedelegationsResponse represents redelegations response.
type RedelegationsResponse struct {
	RedelegationResponses []struct {
		Redelegation struct {
			DelegatorAddress    string `json:"delegatorAddress"`
			ValidatorSrcAddress string `json:"validatorSrcAddress"`
			ValidatorDstAddress string `json:"validatorDstAddress"`
			Entries             []struct {
				CreationHeight string `json:"creationHeight"`
				CompletionTime string `json:"completionTime"`
				InitialBalance string `json:"initialBalance"`
				SharesDst      string `json:"sharesDst"`
			} `json:"entries"`
		} `json:"redelegation"`
		Entries []struct {
			RedelegationEntry struct {
				CreationHeight string `json:"creationHeight"`
				CompletionTime string `json:"completionTime"`
				InitialBalance string `json:"initialBalance"`
				SharesDst      string `json:"sharesDst"`
			} `json:"redelegationEntry"`
			Balance string `json:"balance"`
		} `json:"entries"`
	} `json:"redelegationResponses"`
	Pagination *PaginationResp `json:"pagination,omitempty"`
}

// CommunityPoolResponse represents community pool response.
type CommunityPoolResponse struct {
	Pool []struct {
		Denom  string `json:"denom"`
		Amount string `json:"amount"`
	} `json:"pool"`
}

// DelegationRewardsResponse represents delegation rewards response.
type DelegationRewardsResponse struct {
	Rewards []struct {
		Denom  string `json:"denom"`
		Amount string `json:"amount"`
	} `json:"rewards"`
}

// DelegationTotalRewardsResponse represents total rewards response.
type DelegationTotalRewardsResponse struct {
	Rewards []struct {
		ValidatorAddress string `json:"validatorAddress"`
		Reward           []struct {
			Denom  string `json:"denom"`
			Amount string `json:"amount"`
		} `json:"reward"`
	} `json:"rewards"`
	Total []struct {
		Denom  string `json:"denom"`
		Amount string `json:"amount"`
	} `json:"total"`
}

// WithdrawAddressResponse represents withdraw address response.
type WithdrawAddressResponse struct {
	WithdrawAddress string `json:"withdrawAddress"`
}

// ValidatorCommissionResponse represents validator commission response.
type ValidatorCommissionResponse struct {
	Commission struct {
		Commission []struct {
			Denom  string `json:"denom"`
			Amount string `json:"amount"`
		} `json:"commission"`
	} `json:"commission"`
}

// ValidatorOutstandingRewardsResponse represents outstanding rewards response.
type ValidatorOutstandingRewardsResponse struct {
	Rewards struct {
		Rewards []struct {
			Denom  string `json:"denom"`
			Amount string `json:"amount"`
		} `json:"rewards"`
	} `json:"rewards"`
}

// DistributionParamsResponse represents distribution params response.
type DistributionParamsResponse struct {
	Params struct {
		CommunityTax        string `json:"communityTax"`
		BaseProposerReward  string `json:"baseProposerReward"`
		BonusProposerReward string `json:"bonusProposerReward"`
		WithdrawAddrEnabled bool   `json:"withdrawAddrEnabled"`
	} `json:"params"`
}

// ProposalsResponse represents gov proposals response.
type ProposalsResponse struct {
	Proposals  []json.RawMessage `json:"proposals"`
	Pagination *PaginationResp   `json:"pagination,omitempty"`
}

// ProposalResponse represents a single proposal response.
type ProposalResponse struct {
	Proposal json.RawMessage `json:"proposal"`
}

// VotesResponse represents proposal votes response.
type VotesResponse struct {
	Votes      []json.RawMessage `json:"votes"`
	Pagination *PaginationResp   `json:"pagination,omitempty"`
}

// DepositsResponse represents proposal deposits response.
type DepositsResponse struct {
	Deposits   []json.RawMessage `json:"deposits"`
	Pagination *PaginationResp   `json:"pagination,omitempty"`
}

// TallyResultResponse represents tally result response.
type TallyResultResponse struct {
	Tally struct {
		YesCount        string `json:"yesCount"`
		AbstainCount    string `json:"abstainCount"`
		NoCount         string `json:"noCount"`
		NoWithVetoCount string `json:"noWithVetoCount"`
	} `json:"tally"`
}

// GovParamsResponse represents gov params response.
type GovParamsResponse struct {
	VotingParams  json.RawMessage `json:"votingParams,omitempty"`
	DepositParams json.RawMessage `json:"depositParams,omitempty"`
	TallyParams   json.RawMessage `json:"tallyParams,omitempty"`
	Params        json.RawMessage `json:"params,omitempty"`
}

// InflationResponse represents inflation response.
type InflationResponse struct {
	Inflation string `json:"inflation"`
}

// AnnualProvisionsResponse represents annual provisions response.
type AnnualProvisionsResponse struct {
	AnnualProvisions string `json:"annualProvisions"`
}

// MintParamsResponse represents mint params response.
type MintParamsResponse struct {
	Params struct {
		MintDenom           string `json:"mintDenom"`
		InflationRateChange string `json:"inflationRateChange"`
		InflationMax        string `json:"inflationMax"`
		InflationMin        string `json:"inflationMin"`
		GoalBonded          string `json:"goalBonded"`
		BlocksPerYear       string `json:"blocksPerYear"`
	} `json:"params"`
}

// SigningInfoResponse represents signing info response.
type SigningInfoResponse struct {
	ValSigningInfo struct {
		Address             string `json:"address"`
		StartHeight         string `json:"startHeight"`
		IndexOffset         string `json:"indexOffset"`
		JailedUntil         string `json:"jailedUntil"`
		Tombstoned          bool   `json:"tombstoned"`
		MissedBlocksCounter string `json:"missedBlocksCounter"`
	} `json:"valSigningInfo"`
}

// SigningInfosResponse represents all signing infos response.
type SigningInfosResponse struct {
	Info       []json.RawMessage `json:"info"`
	Pagination *PaginationResp   `json:"pagination,omitempty"`
}

// SlashingParamsResponse represents slashing params response.
type SlashingParamsResponse struct {
	Params struct {
		SignedBlocksWindow      string `json:"signedBlocksWindow"`
		MinSignedPerWindow      string `json:"minSignedPerWindow"`
		DowntimeJailDuration    string `json:"downtimeJailDuration"`
		SlashFractionDoubleSign string `json:"slashFractionDoubleSign"`
		SlashFractionDowntime   string `json:"slashFractionDowntime"`
	} `json:"params"`
}

// EvidenceResponse represents evidence response.
type EvidenceResponse struct {
	Evidence json.RawMessage `json:"evidence"`
}

// AllEvidenceResponse represents all evidence response.
type AllEvidenceResponse struct {
	Evidence   []json.RawMessage `json:"evidence"`
	Pagination *PaginationResp   `json:"pagination,omitempty"`
}

// AllowanceResponse represents fee allowance response.
type AllowanceResponse struct {
	Allowance json.RawMessage `json:"allowance"`
}

// AllowancesResponse represents fee allowances response.
type AllowancesResponse struct {
	Allowances []json.RawMessage `json:"allowances"`
	Pagination *PaginationResp   `json:"pagination,omitempty"`
}

// CurrentPlanResponse represents current upgrade plan response.
type CurrentPlanResponse struct {
	Plan *struct {
		Name   string `json:"name"`
		Height string `json:"height"`
		Info   string `json:"info"`
	} `json:"plan"`
}

// AppliedPlanResponse represents applied plan response.
type AppliedPlanResponse struct {
	Height string `json:"height"`
}

// ModuleVersionsResponse represents module versions response.
type ModuleVersionsResponse struct {
	ModuleVersions []struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"moduleVersions"`
}

// TxResponse represents a single transaction response.
type TxResponse struct {
	Tx         json.RawMessage `json:"tx"`
	TxResponse json.RawMessage `json:"txResponse"`
}

// BlockWithTxsResponse represents block with transactions response.
type BlockWithTxsResponse struct {
	Txs        []json.RawMessage `json:"txs"`
	BlockID    json.RawMessage   `json:"blockId"`
	Block      json.RawMessage   `json:"block"`
	Pagination *PaginationResp   `json:"pagination,omitempty"`
}

// IBCClientStatesResponse represents IBC client states response.
type IBCClientStatesResponse struct {
	ClientStates []struct {
		ClientID    string          `json:"clientId"`
		ClientState json.RawMessage `json:"clientState"`
	} `json:"clientStates"`
	Pagination *PaginationResp `json:"pagination,omitempty"`
}

// IBCClientStateResponse represents a single IBC client state response.
type IBCClientStateResponse struct {
	ClientState json.RawMessage `json:"clientState"`
	ProofHeight struct {
		RevisionNumber string `json:"revisionNumber"`
		RevisionHeight string `json:"revisionHeight"`
	} `json:"proofHeight"`
}

// IBCConnectionsResponse represents IBC connections response.
type IBCConnectionsResponse struct {
	Connections []struct {
		ID           string          `json:"id"`
		ClientID     string          `json:"clientId"`
		Versions     json.RawMessage `json:"versions"`
		State        string          `json:"state"`
		Counterparty struct {
			ClientID     string `json:"clientId"`
			ConnectionID string `json:"connectionId"`
			Prefix       struct {
				KeyPrefix string `json:"keyPrefix"`
			} `json:"prefix"`
		} `json:"counterparty"`
		DelayPeriod string `json:"delayPeriod"`
	} `json:"connections"`
	Pagination *PaginationResp `json:"pagination,omitempty"`
	Height     struct {
		RevisionNumber string `json:"revisionNumber"`
		RevisionHeight string `json:"revisionHeight"`
	} `json:"height"`
}

// IBCConnectionResponse represents a single IBC connection response.
type IBCConnectionResponse struct {
	Connection struct {
		ClientID     string          `json:"clientId"`
		Versions     json.RawMessage `json:"versions"`
		State        string          `json:"state"`
		Counterparty struct {
			ClientID     string `json:"clientId"`
			ConnectionID string `json:"connectionId"`
			Prefix       struct {
				KeyPrefix string `json:"keyPrefix"`
			} `json:"prefix"`
		} `json:"counterparty"`
		DelayPeriod string `json:"delayPeriod"`
	} `json:"connection"`
	ProofHeight struct {
		RevisionNumber string `json:"revisionNumber"`
		RevisionHeight string `json:"revisionHeight"`
	} `json:"proofHeight"`
}

// IBCChannelsResponse represents IBC channels response.
type IBCChannelsResponse struct {
	Channels []struct {
		State          string   `json:"state"`
		Ordering       string   `json:"ordering"`
		Counterparty   struct {
			PortID    string `json:"portId"`
			ChannelID string `json:"channelId"`
		} `json:"counterparty"`
		ConnectionHops []string `json:"connectionHops"`
		Version        string   `json:"version"`
		PortID         string   `json:"portId"`
		ChannelID      string   `json:"channelId"`
	} `json:"channels"`
	Pagination *PaginationResp `json:"pagination,omitempty"`
	Height     struct {
		RevisionNumber string `json:"revisionNumber"`
		RevisionHeight string `json:"revisionHeight"`
	} `json:"height"`
}

// IBCChannelResponse represents a single IBC channel response.
type IBCChannelResponse struct {
	Channel struct {
		State          string   `json:"state"`
		Ordering       string   `json:"ordering"`
		Counterparty   struct {
			PortID    string `json:"portId"`
			ChannelID string `json:"channelId"`
		} `json:"counterparty"`
		ConnectionHops []string `json:"connectionHops"`
		Version        string   `json:"version"`
	} `json:"channel"`
	ProofHeight struct {
		RevisionNumber string `json:"revisionNumber"`
		RevisionHeight string `json:"revisionHeight"`
	} `json:"proofHeight"`
}

// IBCDenomTraceResponse represents IBC denom trace response.
type IBCDenomTraceResponse struct {
	DenomTrace struct {
		Path      string `json:"path"`
		BaseDenom string `json:"baseDenom"`
	} `json:"denomTrace"`
}

// IBCDenomTracesResponse represents IBC denom traces response.
type IBCDenomTracesResponse struct {
	DenomTraces []struct {
		Path      string `json:"path"`
		BaseDenom string `json:"baseDenom"`
	} `json:"denomTraces"`
	Pagination *PaginationResp `json:"pagination,omitempty"`
}

// IBCDenomHashResponse represents IBC denom hash response.
type IBCDenomHashResponse struct {
	Hash string `json:"hash"`
}

// IBCEscrowAddressResponse represents IBC escrow address response.
type IBCEscrowAddressResponse struct {
	EscrowAddress string `json:"escrowAddress"`
}

// IBCTotalEscrowResponse represents total escrow for denom response.
type IBCTotalEscrowResponse struct {
	Amount Coin `json:"amount"`
}

// IBCTransferParamsResponse represents IBC transfer params response.
type IBCTransferParamsResponse struct {
	Params struct {
		SendEnabled    bool `json:"sendEnabled"`
		ReceiveEnabled bool `json:"receiveEnabled"`
	} `json:"params"`
}

// GetLatestBlock fetches the latest block from the chain.
func (c *Client) GetLatestBlock() (*BlockResponse, error) {
	resp, err := c.Invoke(methodGetLatestBlock, []byte("{}"))
	if err != nil {
		return nil, fmt.Errorf("GetLatestBlock: %w", err)
	}

	var block BlockResponse
	if err := json.Unmarshal(resp, &block); err != nil {
		return nil, fmt.Errorf("GetLatestBlock: parse response: %w", err)
	}

	return &block, nil
}

// GetLatestBlockHeight returns the latest block height as an int64.
func (c *Client) GetLatestBlockHeight() (int64, error) {
	block, err := c.GetLatestBlock()
	if err != nil {
		return 0, err
	}

	var height int64
	_, err = fmt.Sscanf(block.Block.Header.Height, "%d", &height)
	if err != nil {
		return 0, fmt.Errorf("GetLatestBlockHeight: parse height: %w", err)
	}

	return height, nil
}

// GetBlockByHeight fetches block information for a given height.
func (c *Client) GetBlockByHeight(height int64) (*BlockResponse, error) {
	request := fmt.Sprintf(`{"height":"%d"}`, height)

	resp, err := c.Invoke(methodGetBlockByHeight, []byte(request))
	if err != nil {
		return nil, fmt.Errorf("GetBlockByHeight(%d): %w", height, err)
	}

	var block BlockResponse
	if err := json.Unmarshal(resp, &block); err != nil {
		return nil, fmt.Errorf("GetBlockByHeight(%d): parse response: %w", height, err)
	}

	return &block, nil
}

// GetTxsByHeight fetches all transactions for a given block height.
// Returns raw JSON response for flexibility in parsing.
func (c *Client) GetTxsByHeight(height int64) ([]byte, error) {
	request := fmt.Sprintf(`{"query":"tx.height=%d"}`, height)

	resp, err := c.Invoke(methodGetTxsEvent, []byte(request))
	if err != nil {
		return nil, fmt.Errorf("GetTxsByHeight(%d): %w", height, err)
	}

	return resp, nil
}

// GetTxsByHeightParsed fetches transactions and returns them in a structured format.
func (c *Client) GetTxsByHeightParsed(height int64) (*TxsEventResponse, error) {
	resp, err := c.GetTxsByHeight(height)
	if err != nil {
		return nil, err
	}

	var parsed TxsEventResponse
	if err := json.Unmarshal(resp, &parsed); err != nil {
		return nil, fmt.Errorf("GetTxsByHeightParsed(%d): parse response: %w", height, err)
	}

	return &parsed, nil
}

// GetNodeInfo fetches node information including chain ID.
func (c *Client) GetNodeInfo() (*NodeInfoResponse, error) {
	resp, err := c.Invoke(methodGetNodeInfo, []byte("{}"))
	if err != nil {
		return nil, fmt.Errorf("GetNodeInfo: %w", err)
	}

	var info NodeInfoResponse
	if err := json.Unmarshal(resp, &info); err != nil {
		return nil, fmt.Errorf("GetNodeInfo: parse response: %w", err)
	}

	return &info, nil
}

// GetChainID extracts the chain ID from node info.
func (c *Client) GetChainID() (string, error) {
	info, err := c.GetNodeInfo()
	if err != nil {
		return "", err
	}

	return info.DefaultNodeInfo.Network, nil
}

// GetBalance fetches the balance of a specific denom for an address.
func (c *Client) GetBalance(address, denom string) (*BalanceResponse, error) {
	request := fmt.Sprintf(`{"address":"%s","denom":"%s"}`, address, denom)

	resp, err := c.Invoke(methodBalance, []byte(request))
	if err != nil {
		return nil, fmt.Errorf("GetBalance(%s, %s): %w", address, denom, err)
	}

	var balance BalanceResponse
	if err := json.Unmarshal(resp, &balance); err != nil {
		return nil, fmt.Errorf("GetBalance: parse response: %w", err)
	}

	return &balance, nil
}

// GetAllBalances fetches all balances for an address.
func (c *Client) GetAllBalances(address string) (*AllBalancesResponse, error) {
	request := fmt.Sprintf(`{"address":"%s"}`, address)

	resp, err := c.Invoke(methodAllBalances, []byte(request))
	if err != nil {
		return nil, fmt.Errorf("GetAllBalances(%s): %w", address, err)
	}

	var balances AllBalancesResponse
	if err := json.Unmarshal(resp, &balances); err != nil {
		return nil, fmt.Errorf("GetAllBalances: parse response: %w", err)
	}

	return &balances, nil
}

// GetValidators fetches validators with optional status filter.
// Status can be: BOND_STATUS_BONDED, BOND_STATUS_UNBONDED, BOND_STATUS_UNBONDING, or empty for all.
func (c *Client) GetValidators(status string) (*ValidatorsResponse, error) {
	var request string
	if status != "" {
		request = fmt.Sprintf(`{"status":"%s"}`, status)
	} else {
		request = "{}"
	}

	resp, err := c.Invoke(methodValidators, []byte(request))
	if err != nil {
		return nil, fmt.Errorf("GetValidators(%s): %w", status, err)
	}

	var validators ValidatorsResponse
	if err := json.Unmarshal(resp, &validators); err != nil {
		return nil, fmt.Errorf("GetValidators: parse response: %w", err)
	}

	return &validators, nil
}

// GetAllValidators fetches all validators (all statuses) with pagination handling.
// Returns a map of operator address -> moniker.
func (c *Client) GetAllValidators() (map[string]string, error) {
	result := make(map[string]string)
	statuses := []string{"BOND_STATUS_BONDED", "BOND_STATUS_UNBONDING", "BOND_STATUS_UNBONDED"}

	for _, status := range statuses {
		var nextKey string

		for {
			var request string
			if nextKey != "" {
				request = fmt.Sprintf(`{"status":"%s","pagination":{"key":"%s"}}`, status, nextKey)
			} else {
				request = fmt.Sprintf(`{"status":"%s"}`, status)
			}

			resp, err := c.Invoke(methodValidators, []byte(request))
			if err != nil {
				return nil, fmt.Errorf("GetAllValidators(%s): %w", status, err)
			}

			var validators ValidatorsResponse
			if err := json.Unmarshal(resp, &validators); err != nil {
				return nil, fmt.Errorf("GetAllValidators: parse response: %w", err)
			}

			for _, v := range validators.Validators {
				result[v.OperatorAddress] = v.Description.Moniker
			}

			if validators.Pagination == nil || validators.Pagination.NextKey == "" {
				break
			}
			nextKey = validators.Pagination.NextKey
		}
	}

	return result, nil
}

// GetModuleAccounts fetches all module accounts.
// Returns a map of address -> module name.
func (c *Client) GetModuleAccounts() (map[string]string, error) {
	resp, err := c.Invoke(methodModuleAccounts, []byte("{}"))
	if err != nil {
		return nil, fmt.Errorf("GetModuleAccounts: %w", err)
	}

	var result ModuleAccountsResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetModuleAccounts: parse response: %w", err)
	}

	modules := make(map[string]string)
	for _, acc := range result.Accounts {
		if acc.BaseAccount.Address != "" && acc.Name != "" {
			modules[acc.BaseAccount.Address] = acc.Name
		}
	}

	return modules, nil
}

// GetStakingParams fetches staking parameters.
func (c *Client) GetStakingParams() (*StakingParamsResponse, error) {
	resp, err := c.Invoke(methodStakingParams, []byte("{}"))
	if err != nil {
		return nil, fmt.Errorf("GetStakingParams: %w", err)
	}

	var params StakingParamsResponse
	if err := json.Unmarshal(resp, &params); err != nil {
		return nil, fmt.Errorf("GetStakingParams: parse response: %w", err)
	}

	return &params, nil
}

// GetBondDenom fetches the bond denom from staking params.
func (c *Client) GetBondDenom() (string, error) {
	params, err := c.GetStakingParams()
	if err != nil {
		return "", err
	}

	return params.Params.BondDenom, nil
}

// GetDelegations fetches delegations for a delegator address.
func (c *Client) GetDelegations(delegatorAddr string) (*DelegationsResponse, error) {
	request := fmt.Sprintf(`{"delegatorAddr":"%s"}`, delegatorAddr)

	resp, err := c.Invoke(methodDelegatorDelegations, []byte(request))
	if err != nil {
		return nil, fmt.Errorf("GetDelegations(%s): %w", delegatorAddr, err)
	}

	var delegations DelegationsResponse
	if err := json.Unmarshal(resp, &delegations); err != nil {
		return nil, fmt.Errorf("GetDelegations: parse response: %w", err)
	}

	return &delegations, nil
}

// GetEarliestBlockHeight finds the earliest available block height.
// First tries to fetch block 1; if pruned, parses error or falls back to binary search.
func (c *Client) GetEarliestBlockHeight() (int64, error) {
	latestHeight, err := c.GetLatestBlockHeight()
	if err != nil {
		return 0, fmt.Errorf("GetEarliestBlockHeight: get latest: %w", err)
	}

	// Try block 1
	_, err = c.GetBlockByHeight(1)
	if err == nil {
		return 1, nil
	}

	// Parse error for earliest available height
	errStr := err.Error()
	if earliest := parseEarliestFromError(errStr); earliest > 0 {
		return earliest, nil
	}

	// Binary search fallback
	return c.binarySearchEarliest(latestHeight)
}

// parseEarliestFromError tries to extract earliest available height from error message.
func parseEarliestFromError(errStr string) int64 {
	patterns := []string{
		"lowest height is ",
		"base height: ",
		"earliest available block height is ",
		"earliest available: ",
		"min height: ",
	}

	for _, pattern := range patterns {
		if idx := strings.Index(errStr, pattern); idx != -1 {
			numStart := idx + len(pattern)
			numEnd := numStart
			for numEnd < len(errStr) && errStr[numEnd] >= '0' && errStr[numEnd] <= '9' {
				numEnd++
			}
			if numEnd > numStart {
				var height int64
				fmt.Sscanf(errStr[numStart:numEnd], "%d", &height)
				if height > 0 {
					return height
				}
			}
		}
	}
	return 0
}

// binarySearchEarliest performs binary search to find the earliest available block.
func (c *Client) binarySearchEarliest(latestHeight int64) (int64, error) {
	// Verify latest block is accessible
	_, err := c.GetBlockByHeight(latestHeight)
	if err != nil {
		return 0, fmt.Errorf("latest block %d not accessible: %w", latestHeight, err)
	}

	low, high := int64(1), latestHeight
	earliest := latestHeight

	for low <= high {
		mid := (low + high) / 2
		_, err := c.GetBlockByHeight(mid)
		if err == nil {
			earliest = mid
			high = mid - 1
		} else {
			low = mid + 1
		}
	}

	return earliest, nil
}

// ============================================================================
// Tendermint/CometBFT Service Methods
// ============================================================================

// GetSyncing returns whether the node is currently syncing.
func (c *Client) GetSyncing() (bool, error) {
	resp, err := c.Invoke(methodGetSyncing, []byte("{}"))
	if err != nil {
		return false, fmt.Errorf("GetSyncing: %w", err)
	}

	var result SyncingResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return false, fmt.Errorf("GetSyncing: parse response: %w", err)
	}

	return result.Syncing, nil
}

// GetLatestValidatorSet fetches the latest validator set.
func (c *Client) GetLatestValidatorSet() (*ValidatorSetResponse, error) {
	resp, err := c.Invoke(methodGetLatestValidatorSet, []byte("{}"))
	if err != nil {
		return nil, fmt.Errorf("GetLatestValidatorSet: %w", err)
	}

	var result ValidatorSetResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetLatestValidatorSet: parse response: %w", err)
	}

	return &result, nil
}

// GetValidatorSetByHeight fetches the validator set at a specific height.
func (c *Client) GetValidatorSetByHeight(height int64) (*ValidatorSetResponse, error) {
	request := fmt.Sprintf(`{"height":"%d"}`, height)

	resp, err := c.Invoke(methodGetValidatorSetByH, []byte(request))
	if err != nil {
		return nil, fmt.Errorf("GetValidatorSetByHeight(%d): %w", height, err)
	}

	var result ValidatorSetResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetValidatorSetByHeight: parse response: %w", err)
	}

	return &result, nil
}

// ============================================================================
// Transaction Service Methods
// ============================================================================

// GetTx fetches a transaction by hash.
func (c *Client) GetTx(hash string) (*TxResponse, error) {
	request := fmt.Sprintf(`{"hash":"%s"}`, hash)

	resp, err := c.Invoke(methodGetTx, []byte(request))
	if err != nil {
		return nil, fmt.Errorf("GetTx(%s): %w", hash, err)
	}

	var result TxResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetTx: parse response: %w", err)
	}

	return &result, nil
}

// GetBlockWithTxs fetches a block with all transactions at a specific height.
func (c *Client) GetBlockWithTxs(height int64) (*BlockWithTxsResponse, error) {
	request := fmt.Sprintf(`{"height":"%d"}`, height)

	resp, err := c.Invoke(methodGetBlockWithTxs, []byte(request))
	if err != nil {
		return nil, fmt.Errorf("GetBlockWithTxs(%d): %w", height, err)
	}

	var result BlockWithTxsResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetBlockWithTxs: parse response: %w", err)
	}

	return &result, nil
}

// ============================================================================
// Auth Module Methods
// ============================================================================

// GetAccount fetches account information by address.
func (c *Client) GetAccount(address string) (*AccountResponse, error) {
	request := fmt.Sprintf(`{"address":"%s"}`, address)

	resp, err := c.Invoke(methodAccount, []byte(request))
	if err != nil {
		return nil, fmt.Errorf("GetAccount(%s): %w", address, err)
	}

	var result AccountResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetAccount: parse response: %w", err)
	}

	return &result, nil
}

// GetAccounts fetches all accounts with pagination.
func (c *Client) GetAccounts(paginationKey string) (*AccountsResponse, error) {
	var request string
	if paginationKey != "" {
		request = fmt.Sprintf(`{"pagination":{"key":"%s"}}`, paginationKey)
	} else {
		request = "{}"
	}

	resp, err := c.Invoke(methodAccounts, []byte(request))
	if err != nil {
		return nil, fmt.Errorf("GetAccounts: %w", err)
	}

	var result AccountsResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetAccounts: parse response: %w", err)
	}

	return &result, nil
}

// GetAccountInfo fetches account info (address, pubkey, account number, sequence).
func (c *Client) GetAccountInfo(address string) (*AccountInfoResponse, error) {
	request := fmt.Sprintf(`{"address":"%s"}`, address)

	resp, err := c.Invoke(methodAccountInfo, []byte(request))
	if err != nil {
		return nil, fmt.Errorf("GetAccountInfo(%s): %w", address, err)
	}

	var result AccountInfoResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetAccountInfo: parse response: %w", err)
	}

	return &result, nil
}

// GetModuleAccountByName fetches a module account by name.
func (c *Client) GetModuleAccountByName(name string) (*AccountResponse, error) {
	request := fmt.Sprintf(`{"name":"%s"}`, name)

	resp, err := c.Invoke(methodModuleAccountByN, []byte(request))
	if err != nil {
		return nil, fmt.Errorf("GetModuleAccountByName(%s): %w", name, err)
	}

	var result AccountResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetModuleAccountByName: parse response: %w", err)
	}

	return &result, nil
}

// GetBech32Prefix fetches the bech32 prefix used by the chain.
func (c *Client) GetBech32Prefix() (string, error) {
	resp, err := c.Invoke(methodBech32Prefix, []byte("{}"))
	if err != nil {
		return "", fmt.Errorf("GetBech32Prefix: %w", err)
	}

	var result Bech32PrefixResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", fmt.Errorf("GetBech32Prefix: parse response: %w", err)
	}

	return result.Bech32Prefix, nil
}

// GetAuthParams fetches auth module parameters.
func (c *Client) GetAuthParams() (*AuthParamsResponse, error) {
	resp, err := c.Invoke(methodAuthParams, []byte("{}"))
	if err != nil {
		return nil, fmt.Errorf("GetAuthParams: %w", err)
	}

	var result AuthParamsResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetAuthParams: parse response: %w", err)
	}

	return &result, nil
}

// ============================================================================
// Authz Module Methods
// ============================================================================

// GetGrants fetches grants for a granter/grantee pair.
func (c *Client) GetGrants(granter, grantee string) (*GrantsResponse, error) {
	request := fmt.Sprintf(`{"granter":"%s","grantee":"%s"}`, granter, grantee)

	resp, err := c.Invoke(methodGrants, []byte(request))
	if err != nil {
		return nil, fmt.Errorf("GetGrants: %w", err)
	}

	var result GrantsResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetGrants: parse response: %w", err)
	}

	return &result, nil
}

// GetGranterGrants fetches all grants given by a granter.
func (c *Client) GetGranterGrants(granter string) (*GrantsResponse, error) {
	request := fmt.Sprintf(`{"granter":"%s"}`, granter)

	resp, err := c.Invoke(methodGranterGrants, []byte(request))
	if err != nil {
		return nil, fmt.Errorf("GetGranterGrants(%s): %w", granter, err)
	}

	var result GrantsResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetGranterGrants: parse response: %w", err)
	}

	return &result, nil
}

// GetGranteeGrants fetches all grants received by a grantee.
func (c *Client) GetGranteeGrants(grantee string) (*GrantsResponse, error) {
	request := fmt.Sprintf(`{"grantee":"%s"}`, grantee)

	resp, err := c.Invoke(methodGranteeGrants, []byte(request))
	if err != nil {
		return nil, fmt.Errorf("GetGranteeGrants(%s): %w", grantee, err)
	}

	var result GrantsResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetGranteeGrants: parse response: %w", err)
	}

	return &result, nil
}

// ============================================================================
// Bank Module Methods
// ============================================================================

// GetTotalSupply fetches the total supply of all coins.
func (c *Client) GetTotalSupply() (*TotalSupplyResponse, error) {
	resp, err := c.Invoke(methodTotalSupply, []byte("{}"))
	if err != nil {
		return nil, fmt.Errorf("GetTotalSupply: %w", err)
	}

	var result TotalSupplyResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetTotalSupply: parse response: %w", err)
	}

	return &result, nil
}

// GetSupplyOf fetches the supply of a specific denomination.
func (c *Client) GetSupplyOf(denom string) (*SupplyOfResponse, error) {
	request := fmt.Sprintf(`{"denom":"%s"}`, denom)

	resp, err := c.Invoke(methodSupplyOf, []byte(request))
	if err != nil {
		return nil, fmt.Errorf("GetSupplyOf(%s): %w", denom, err)
	}

	var result SupplyOfResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetSupplyOf: parse response: %w", err)
	}

	return &result, nil
}

// GetSpendableBalances fetches spendable balances for an address.
func (c *Client) GetSpendableBalances(address string) (*SpendableBalancesResponse, error) {
	request := fmt.Sprintf(`{"address":"%s"}`, address)

	resp, err := c.Invoke(methodSpendableBalance, []byte(request))
	if err != nil {
		return nil, fmt.Errorf("GetSpendableBalances(%s): %w", address, err)
	}

	var result SpendableBalancesResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetSpendableBalances: parse response: %w", err)
	}

	return &result, nil
}

// GetDenomMetadata fetches metadata for a specific denomination.
func (c *Client) GetDenomMetadata(denom string) (*DenomMetadataResponse, error) {
	request := fmt.Sprintf(`{"denom":"%s"}`, denom)

	resp, err := c.Invoke(methodDenomMetadata, []byte(request))
	if err != nil {
		return nil, fmt.Errorf("GetDenomMetadata(%s): %w", denom, err)
	}

	var result DenomMetadataResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetDenomMetadata: parse response: %w", err)
	}

	return &result, nil
}

// GetDenomsMetadata fetches metadata for all denominations.
func (c *Client) GetDenomsMetadata() (*DenomsMetadataResponse, error) {
	resp, err := c.Invoke(methodDenomsMetadata, []byte("{}"))
	if err != nil {
		return nil, fmt.Errorf("GetDenomsMetadata: %w", err)
	}

	var result DenomsMetadataResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetDenomsMetadata: parse response: %w", err)
	}

	return &result, nil
}

// GetDenomOwners fetches all owners of a specific denomination.
func (c *Client) GetDenomOwners(denom string) (*DenomOwnersResponse, error) {
	request := fmt.Sprintf(`{"denom":"%s"}`, denom)

	resp, err := c.Invoke(methodDenomOwners, []byte(request))
	if err != nil {
		return nil, fmt.Errorf("GetDenomOwners(%s): %w", denom, err)
	}

	var result DenomOwnersResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetDenomOwners: parse response: %w", err)
	}

	return &result, nil
}

// GetBankParams fetches bank module parameters.
func (c *Client) GetBankParams() (*BankParamsResponse, error) {
	resp, err := c.Invoke(methodBankParams, []byte("{}"))
	if err != nil {
		return nil, fmt.Errorf("GetBankParams: %w", err)
	}

	var result BankParamsResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetBankParams: parse response: %w", err)
	}

	return &result, nil
}

// ============================================================================
// Staking Module Methods
// ============================================================================

// GetValidator fetches a single validator by operator address.
func (c *Client) GetValidator(validatorAddr string) (*ValidatorResponse, error) {
	request := fmt.Sprintf(`{"validatorAddr":"%s"}`, validatorAddr)

	resp, err := c.Invoke(methodValidator, []byte(request))
	if err != nil {
		return nil, fmt.Errorf("GetValidator(%s): %w", validatorAddr, err)
	}

	var result ValidatorResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetValidator: parse response: %w", err)
	}

	return &result, nil
}

// GetStakingPool fetches the current staking pool info (bonded/not bonded tokens).
func (c *Client) GetStakingPool() (*StakingPoolResponse, error) {
	resp, err := c.Invoke(methodStakingPool, []byte("{}"))
	if err != nil {
		return nil, fmt.Errorf("GetStakingPool: %w", err)
	}

	var result StakingPoolResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetStakingPool: parse response: %w", err)
	}

	return &result, nil
}

// GetDelegation fetches a specific delegation.
func (c *Client) GetDelegation(delegatorAddr, validatorAddr string) (*DelegationResponse, error) {
	request := fmt.Sprintf(`{"delegatorAddr":"%s","validatorAddr":"%s"}`, delegatorAddr, validatorAddr)

	resp, err := c.Invoke(methodDelegation, []byte(request))
	if err != nil {
		return nil, fmt.Errorf("GetDelegation: %w", err)
	}

	var result DelegationResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetDelegation: parse response: %w", err)
	}

	return &result, nil
}

// GetUnbondingDelegation fetches a specific unbonding delegation.
func (c *Client) GetUnbondingDelegation(delegatorAddr, validatorAddr string) (*UnbondingDelegationResponse, error) {
	request := fmt.Sprintf(`{"delegatorAddr":"%s","validatorAddr":"%s"}`, delegatorAddr, validatorAddr)

	resp, err := c.Invoke(methodUnbondingDelegation, []byte(request))
	if err != nil {
		return nil, fmt.Errorf("GetUnbondingDelegation: %w", err)
	}

	var result UnbondingDelegationResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetUnbondingDelegation: parse response: %w", err)
	}

	return &result, nil
}

// GetDelegatorUnbondingDelegations fetches all unbonding delegations for a delegator.
func (c *Client) GetDelegatorUnbondingDelegations(delegatorAddr string) (*UnbondingDelegationsResponse, error) {
	request := fmt.Sprintf(`{"delegatorAddr":"%s"}`, delegatorAddr)

	resp, err := c.Invoke(methodDelegatorUnbonding, []byte(request))
	if err != nil {
		return nil, fmt.Errorf("GetDelegatorUnbondingDelegations(%s): %w", delegatorAddr, err)
	}

	var result UnbondingDelegationsResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetDelegatorUnbondingDelegations: parse response: %w", err)
	}

	return &result, nil
}

// GetRedelegations fetches redelegations for a delegator.
func (c *Client) GetRedelegations(delegatorAddr string) (*RedelegationsResponse, error) {
	request := fmt.Sprintf(`{"delegatorAddr":"%s"}`, delegatorAddr)

	resp, err := c.Invoke(methodRedelegations, []byte(request))
	if err != nil {
		return nil, fmt.Errorf("GetRedelegations(%s): %w", delegatorAddr, err)
	}

	var result RedelegationsResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetRedelegations: parse response: %w", err)
	}

	return &result, nil
}

// GetDelegatorValidators fetches all validators a delegator is delegating to.
func (c *Client) GetDelegatorValidators(delegatorAddr string) (*ValidatorsResponse, error) {
	request := fmt.Sprintf(`{"delegatorAddr":"%s"}`, delegatorAddr)

	resp, err := c.Invoke(methodDelegatorValidators, []byte(request))
	if err != nil {
		return nil, fmt.Errorf("GetDelegatorValidators(%s): %w", delegatorAddr, err)
	}

	var result ValidatorsResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetDelegatorValidators: parse response: %w", err)
	}

	return &result, nil
}

// GetValidatorDelegations fetches all delegations to a validator.
func (c *Client) GetValidatorDelegations(validatorAddr string) (*DelegationsResponse, error) {
	request := fmt.Sprintf(`{"validatorAddr":"%s"}`, validatorAddr)

	resp, err := c.Invoke(methodValidatorDelegations, []byte(request))
	if err != nil {
		return nil, fmt.Errorf("GetValidatorDelegations(%s): %w", validatorAddr, err)
	}

	var result DelegationsResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetValidatorDelegations: parse response: %w", err)
	}

	return &result, nil
}

// ============================================================================
// Distribution Module Methods
// ============================================================================

// GetCommunityPool fetches the community pool balance.
func (c *Client) GetCommunityPool() (*CommunityPoolResponse, error) {
	resp, err := c.Invoke(methodCommunityPool, []byte("{}"))
	if err != nil {
		return nil, fmt.Errorf("GetCommunityPool: %w", err)
	}

	var result CommunityPoolResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetCommunityPool: parse response: %w", err)
	}

	return &result, nil
}

// GetDelegationRewards fetches rewards for a specific delegation.
func (c *Client) GetDelegationRewards(delegatorAddr, validatorAddr string) (*DelegationRewardsResponse, error) {
	request := fmt.Sprintf(`{"delegatorAddress":"%s","validatorAddress":"%s"}`, delegatorAddr, validatorAddr)

	resp, err := c.Invoke(methodDelegationRewards, []byte(request))
	if err != nil {
		return nil, fmt.Errorf("GetDelegationRewards: %w", err)
	}

	var result DelegationRewardsResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetDelegationRewards: parse response: %w", err)
	}

	return &result, nil
}

// GetDelegationTotalRewards fetches total rewards for all delegations of a delegator.
func (c *Client) GetDelegationTotalRewards(delegatorAddr string) (*DelegationTotalRewardsResponse, error) {
	request := fmt.Sprintf(`{"delegatorAddress":"%s"}`, delegatorAddr)

	resp, err := c.Invoke(methodDelegationTotalRewards, []byte(request))
	if err != nil {
		return nil, fmt.Errorf("GetDelegationTotalRewards(%s): %w", delegatorAddr, err)
	}

	var result DelegationTotalRewardsResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetDelegationTotalRewards: parse response: %w", err)
	}

	return &result, nil
}

// GetDelegatorWithdrawAddress fetches the withdraw address for a delegator.
func (c *Client) GetDelegatorWithdrawAddress(delegatorAddr string) (string, error) {
	request := fmt.Sprintf(`{"delegatorAddress":"%s"}`, delegatorAddr)

	resp, err := c.Invoke(methodDelegatorWithdrawAddr, []byte(request))
	if err != nil {
		return "", fmt.Errorf("GetDelegatorWithdrawAddress(%s): %w", delegatorAddr, err)
	}

	var result WithdrawAddressResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", fmt.Errorf("GetDelegatorWithdrawAddress: parse response: %w", err)
	}

	return result.WithdrawAddress, nil
}

// GetValidatorCommission fetches accumulated commission for a validator.
func (c *Client) GetValidatorCommission(validatorAddr string) (*ValidatorCommissionResponse, error) {
	request := fmt.Sprintf(`{"validatorAddress":"%s"}`, validatorAddr)

	resp, err := c.Invoke(methodValidatorCommission, []byte(request))
	if err != nil {
		return nil, fmt.Errorf("GetValidatorCommission(%s): %w", validatorAddr, err)
	}

	var result ValidatorCommissionResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetValidatorCommission: parse response: %w", err)
	}

	return &result, nil
}

// GetValidatorOutstandingRewards fetches outstanding rewards for a validator.
func (c *Client) GetValidatorOutstandingRewards(validatorAddr string) (*ValidatorOutstandingRewardsResponse, error) {
	request := fmt.Sprintf(`{"validatorAddress":"%s"}`, validatorAddr)

	resp, err := c.Invoke(methodValidatorOutstanding, []byte(request))
	if err != nil {
		return nil, fmt.Errorf("GetValidatorOutstandingRewards(%s): %w", validatorAddr, err)
	}

	var result ValidatorOutstandingRewardsResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetValidatorOutstandingRewards: parse response: %w", err)
	}

	return &result, nil
}

// GetDistributionParams fetches distribution module parameters.
func (c *Client) GetDistributionParams() (*DistributionParamsResponse, error) {
	resp, err := c.Invoke(methodDistributionParams, []byte("{}"))
	if err != nil {
		return nil, fmt.Errorf("GetDistributionParams: %w", err)
	}

	var result DistributionParamsResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetDistributionParams: parse response: %w", err)
	}

	return &result, nil
}

// ============================================================================
// Gov Module Methods (v1)
// ============================================================================

// GetProposals fetches all governance proposals.
// Status can be: PROPOSAL_STATUS_UNSPECIFIED, PROPOSAL_STATUS_DEPOSIT_PERIOD,
// PROPOSAL_STATUS_VOTING_PERIOD, PROPOSAL_STATUS_PASSED, PROPOSAL_STATUS_REJECTED,
// PROPOSAL_STATUS_FAILED, or empty for all.
func (c *Client) GetProposals(status string) (*ProposalsResponse, error) {
	var request string
	if status != "" {
		request = fmt.Sprintf(`{"proposalStatus":"%s"}`, status)
	} else {
		request = "{}"
	}

	resp, err := c.Invoke(methodProposals, []byte(request))
	if err != nil {
		return nil, fmt.Errorf("GetProposals: %w", err)
	}

	var result ProposalsResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetProposals: parse response: %w", err)
	}

	return &result, nil
}

// GetProposal fetches a single governance proposal by ID.
func (c *Client) GetProposal(proposalID uint64) (*ProposalResponse, error) {
	request := fmt.Sprintf(`{"proposalId":"%d"}`, proposalID)

	resp, err := c.Invoke(methodProposal, []byte(request))
	if err != nil {
		return nil, fmt.Errorf("GetProposal(%d): %w", proposalID, err)
	}

	var result ProposalResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetProposal: parse response: %w", err)
	}

	return &result, nil
}

// GetProposalVotes fetches votes for a proposal.
func (c *Client) GetProposalVotes(proposalID uint64) (*VotesResponse, error) {
	request := fmt.Sprintf(`{"proposalId":"%d"}`, proposalID)

	resp, err := c.Invoke(methodProposalV, []byte(request))
	if err != nil {
		return nil, fmt.Errorf("GetProposalVotes(%d): %w", proposalID, err)
	}

	var result VotesResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetProposalVotes: parse response: %w", err)
	}

	return &result, nil
}

// GetProposalDeposits fetches deposits for a proposal.
func (c *Client) GetProposalDeposits(proposalID uint64) (*DepositsResponse, error) {
	request := fmt.Sprintf(`{"proposalId":"%d"}`, proposalID)

	resp, err := c.Invoke(methodProposalD, []byte(request))
	if err != nil {
		return nil, fmt.Errorf("GetProposalDeposits(%d): %w", proposalID, err)
	}

	var result DepositsResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetProposalDeposits: parse response: %w", err)
	}

	return &result, nil
}

// GetTallyResult fetches the tally result for a proposal.
func (c *Client) GetTallyResult(proposalID uint64) (*TallyResultResponse, error) {
	request := fmt.Sprintf(`{"proposalId":"%d"}`, proposalID)

	resp, err := c.Invoke(methodTallyResult, []byte(request))
	if err != nil {
		return nil, fmt.Errorf("GetTallyResult(%d): %w", proposalID, err)
	}

	var result TallyResultResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetTallyResult: parse response: %w", err)
	}

	return &result, nil
}

// GetGovParams fetches governance module parameters.
func (c *Client) GetGovParams() (*GovParamsResponse, error) {
	resp, err := c.Invoke(methodGovParams, []byte("{}"))
	if err != nil {
		return nil, fmt.Errorf("GetGovParams: %w", err)
	}

	var result GovParamsResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetGovParams: parse response: %w", err)
	}

	return &result, nil
}

// ============================================================================
// Mint Module Methods
// ============================================================================

// GetInflation fetches the current inflation rate.
func (c *Client) GetInflation() (string, error) {
	resp, err := c.Invoke(methodInflation, []byte("{}"))
	if err != nil {
		return "", fmt.Errorf("GetInflation: %w", err)
	}

	var result InflationResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", fmt.Errorf("GetInflation: parse response: %w", err)
	}

	return result.Inflation, nil
}

// GetAnnualProvisions fetches the current annual provisions.
func (c *Client) GetAnnualProvisions() (string, error) {
	resp, err := c.Invoke(methodAnnualProvisions, []byte("{}"))
	if err != nil {
		return "", fmt.Errorf("GetAnnualProvisions: %w", err)
	}

	var result AnnualProvisionsResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", fmt.Errorf("GetAnnualProvisions: parse response: %w", err)
	}

	return result.AnnualProvisions, nil
}

// GetMintParams fetches mint module parameters.
func (c *Client) GetMintParams() (*MintParamsResponse, error) {
	resp, err := c.Invoke(methodMintParams, []byte("{}"))
	if err != nil {
		return nil, fmt.Errorf("GetMintParams: %w", err)
	}

	var result MintParamsResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetMintParams: parse response: %w", err)
	}

	return &result, nil
}

// ============================================================================
// Slashing Module Methods
// ============================================================================

// GetSigningInfo fetches signing info for a validator consensus address.
func (c *Client) GetSigningInfo(consAddr string) (*SigningInfoResponse, error) {
	request := fmt.Sprintf(`{"consAddress":"%s"}`, consAddr)

	resp, err := c.Invoke(methodSigningInfo, []byte(request))
	if err != nil {
		return nil, fmt.Errorf("GetSigningInfo(%s): %w", consAddr, err)
	}

	var result SigningInfoResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetSigningInfo: parse response: %w", err)
	}

	return &result, nil
}

// GetSigningInfos fetches signing info for all validators.
func (c *Client) GetSigningInfos() (*SigningInfosResponse, error) {
	resp, err := c.Invoke(methodSigningInfos, []byte("{}"))
	if err != nil {
		return nil, fmt.Errorf("GetSigningInfos: %w", err)
	}

	var result SigningInfosResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetSigningInfos: parse response: %w", err)
	}

	return &result, nil
}

// GetSlashingParams fetches slashing module parameters.
func (c *Client) GetSlashingParams() (*SlashingParamsResponse, error) {
	resp, err := c.Invoke(methodSlashingParam, []byte("{}"))
	if err != nil {
		return nil, fmt.Errorf("GetSlashingParams: %w", err)
	}

	var result SlashingParamsResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetSlashingParams: parse response: %w", err)
	}

	return &result, nil
}

// ============================================================================
// Evidence Module Methods
// ============================================================================

// GetEvidence fetches a specific evidence by hash.
func (c *Client) GetEvidence(hash string) (*EvidenceResponse, error) {
	request := fmt.Sprintf(`{"hash":"%s"}`, hash)

	resp, err := c.Invoke(methodEvidence, []byte(request))
	if err != nil {
		return nil, fmt.Errorf("GetEvidence(%s): %w", hash, err)
	}

	var result EvidenceResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetEvidence: parse response: %w", err)
	}

	return &result, nil
}

// GetAllEvidence fetches all evidence.
func (c *Client) GetAllEvidence() (*AllEvidenceResponse, error) {
	resp, err := c.Invoke(methodAllEvidence, []byte("{}"))
	if err != nil {
		return nil, fmt.Errorf("GetAllEvidence: %w", err)
	}

	var result AllEvidenceResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetAllEvidence: parse response: %w", err)
	}

	return &result, nil
}

// ============================================================================
// Feegrant Module Methods
// ============================================================================

// GetAllowance fetches the fee allowance of a grantee by granter.
func (c *Client) GetAllowance(granter, grantee string) (*AllowanceResponse, error) {
	request := fmt.Sprintf(`{"granter":"%s","grantee":"%s"}`, granter, grantee)

	resp, err := c.Invoke(methodAllowance, []byte(request))
	if err != nil {
		return nil, fmt.Errorf("GetAllowance: %w", err)
	}

	var result AllowanceResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetAllowance: parse response: %w", err)
	}

	return &result, nil
}

// GetAllowances fetches all allowances for a grantee.
func (c *Client) GetAllowances(grantee string) (*AllowancesResponse, error) {
	request := fmt.Sprintf(`{"grantee":"%s"}`, grantee)

	resp, err := c.Invoke(methodAllowances, []byte(request))
	if err != nil {
		return nil, fmt.Errorf("GetAllowances(%s): %w", grantee, err)
	}

	var result AllowancesResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetAllowances: parse response: %w", err)
	}

	return &result, nil
}

// GetAllowancesByGranter fetches all allowances given by a granter.
func (c *Client) GetAllowancesByGranter(granter string) (*AllowancesResponse, error) {
	request := fmt.Sprintf(`{"granter":"%s"}`, granter)

	resp, err := c.Invoke(methodAllowancesByGrantr, []byte(request))
	if err != nil {
		return nil, fmt.Errorf("GetAllowancesByGranter(%s): %w", granter, err)
	}

	var result AllowancesResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetAllowancesByGranter: parse response: %w", err)
	}

	return &result, nil
}

// ============================================================================
// Upgrade Module Methods
// ============================================================================

// GetCurrentPlan fetches the current upgrade plan (if any).
func (c *Client) GetCurrentPlan() (*CurrentPlanResponse, error) {
	resp, err := c.Invoke(methodCurrentPlan, []byte("{}"))
	if err != nil {
		return nil, fmt.Errorf("GetCurrentPlan: %w", err)
	}

	var result CurrentPlanResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetCurrentPlan: parse response: %w", err)
	}

	return &result, nil
}

// GetAppliedPlan fetches the height at which a plan was applied.
func (c *Client) GetAppliedPlan(name string) (*AppliedPlanResponse, error) {
	request := fmt.Sprintf(`{"name":"%s"}`, name)

	resp, err := c.Invoke(methodAppliedPlan, []byte(request))
	if err != nil {
		return nil, fmt.Errorf("GetAppliedPlan(%s): %w", name, err)
	}

	var result AppliedPlanResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetAppliedPlan: parse response: %w", err)
	}

	return &result, nil
}

// GetModuleVersions fetches module versions.
func (c *Client) GetModuleVersions() (*ModuleVersionsResponse, error) {
	resp, err := c.Invoke(methodModuleVersions, []byte("{}"))
	if err != nil {
		return nil, fmt.Errorf("GetModuleVersions: %w", err)
	}

	var result ModuleVersionsResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetModuleVersions: parse response: %w", err)
	}

	return &result, nil
}

// ============================================================================
// IBC Core - Client Methods
// ============================================================================

// GetIBCClientStates fetches all IBC client states.
func (c *Client) GetIBCClientStates() (*IBCClientStatesResponse, error) {
	resp, err := c.Invoke(methodClientStates, []byte("{}"))
	if err != nil {
		return nil, fmt.Errorf("GetIBCClientStates: %w", err)
	}

	var result IBCClientStatesResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetIBCClientStates: parse response: %w", err)
	}

	return &result, nil
}

// GetIBCClientState fetches a specific IBC client state.
func (c *Client) GetIBCClientState(clientID string) (*IBCClientStateResponse, error) {
	request := fmt.Sprintf(`{"clientId":"%s"}`, clientID)

	resp, err := c.Invoke(methodClientState, []byte(request))
	if err != nil {
		return nil, fmt.Errorf("GetIBCClientState(%s): %w", clientID, err)
	}

	var result IBCClientStateResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetIBCClientState: parse response: %w", err)
	}

	return &result, nil
}

// ============================================================================
// IBC Core - Connection Methods
// ============================================================================

// GetIBCConnections fetches all IBC connections.
func (c *Client) GetIBCConnections() (*IBCConnectionsResponse, error) {
	resp, err := c.Invoke(methodConnections, []byte("{}"))
	if err != nil {
		return nil, fmt.Errorf("GetIBCConnections: %w", err)
	}

	var result IBCConnectionsResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetIBCConnections: parse response: %w", err)
	}

	return &result, nil
}

// GetIBCConnection fetches a specific IBC connection.
func (c *Client) GetIBCConnection(connectionID string) (*IBCConnectionResponse, error) {
	request := fmt.Sprintf(`{"connectionId":"%s"}`, connectionID)

	resp, err := c.Invoke(methodConnection, []byte(request))
	if err != nil {
		return nil, fmt.Errorf("GetIBCConnection(%s): %w", connectionID, err)
	}

	var result IBCConnectionResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetIBCConnection: parse response: %w", err)
	}

	return &result, nil
}

// GetIBCClientConnections fetches all connections for a client.
func (c *Client) GetIBCClientConnections(clientID string) ([]string, error) {
	request := fmt.Sprintf(`{"clientId":"%s"}`, clientID)

	resp, err := c.Invoke(methodClientConnections, []byte(request))
	if err != nil {
		return nil, fmt.Errorf("GetIBCClientConnections(%s): %w", clientID, err)
	}

	var result struct {
		ConnectionPaths []string `json:"connectionPaths"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetIBCClientConnections: parse response: %w", err)
	}

	return result.ConnectionPaths, nil
}

// ============================================================================
// IBC Core - Channel Methods
// ============================================================================

// GetIBCChannels fetches all IBC channels.
func (c *Client) GetIBCChannels() (*IBCChannelsResponse, error) {
	resp, err := c.Invoke(methodChannels, []byte("{}"))
	if err != nil {
		return nil, fmt.Errorf("GetIBCChannels: %w", err)
	}

	var result IBCChannelsResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetIBCChannels: parse response: %w", err)
	}

	return &result, nil
}

// GetIBCChannel fetches a specific IBC channel.
func (c *Client) GetIBCChannel(portID, channelID string) (*IBCChannelResponse, error) {
	request := fmt.Sprintf(`{"portId":"%s","channelId":"%s"}`, portID, channelID)

	resp, err := c.Invoke(methodChannel, []byte(request))
	if err != nil {
		return nil, fmt.Errorf("GetIBCChannel(%s/%s): %w", portID, channelID, err)
	}

	var result IBCChannelResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetIBCChannel: parse response: %w", err)
	}

	return &result, nil
}

// GetIBCConnectionChannels fetches all channels for a connection.
func (c *Client) GetIBCConnectionChannels(connectionID string) (*IBCChannelsResponse, error) {
	request := fmt.Sprintf(`{"connection":"%s"}`, connectionID)

	resp, err := c.Invoke(methodConnectionChannels, []byte(request))
	if err != nil {
		return nil, fmt.Errorf("GetIBCConnectionChannels(%s): %w", connectionID, err)
	}

	var result IBCChannelsResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetIBCConnectionChannels: parse response: %w", err)
	}

	return &result, nil
}

// ============================================================================
// IBC Applications - Transfer Methods
// ============================================================================

// GetIBCDenomTrace fetches the denom trace for an IBC denom hash.
func (c *Client) GetIBCDenomTrace(hash string) (*IBCDenomTraceResponse, error) {
	request := fmt.Sprintf(`{"hash":"%s"}`, hash)

	resp, err := c.Invoke(methodDenomTrace, []byte(request))
	if err != nil {
		return nil, fmt.Errorf("GetIBCDenomTrace(%s): %w", hash, err)
	}

	var result IBCDenomTraceResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetIBCDenomTrace: parse response: %w", err)
	}

	return &result, nil
}

// GetIBCDenomTraces fetches all denom traces.
func (c *Client) GetIBCDenomTraces() (*IBCDenomTracesResponse, error) {
	resp, err := c.Invoke(methodDenomTraces, []byte("{}"))
	if err != nil {
		return nil, fmt.Errorf("GetIBCDenomTraces: %w", err)
	}

	var result IBCDenomTracesResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetIBCDenomTraces: parse response: %w", err)
	}

	return &result, nil
}

// GetIBCDenomHash calculates the hash for a denom trace path.
func (c *Client) GetIBCDenomHash(trace string) (string, error) {
	request := fmt.Sprintf(`{"trace":"%s"}`, trace)

	resp, err := c.Invoke(methodDenomHash, []byte(request))
	if err != nil {
		return "", fmt.Errorf("GetIBCDenomHash(%s): %w", trace, err)
	}

	var result IBCDenomHashResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", fmt.Errorf("GetIBCDenomHash: parse response: %w", err)
	}

	return result.Hash, nil
}

// GetIBCEscrowAddress fetches the escrow address for a channel.
func (c *Client) GetIBCEscrowAddress(portID, channelID string) (string, error) {
	request := fmt.Sprintf(`{"portId":"%s","channelId":"%s"}`, portID, channelID)

	resp, err := c.Invoke(methodEscrowAddress, []byte(request))
	if err != nil {
		return "", fmt.Errorf("GetIBCEscrowAddress(%s/%s): %w", portID, channelID, err)
	}

	var result IBCEscrowAddressResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", fmt.Errorf("GetIBCEscrowAddress: parse response: %w", err)
	}

	return result.EscrowAddress, nil
}

// GetIBCTotalEscrowForDenom fetches total amount escrowed for a denom.
func (c *Client) GetIBCTotalEscrowForDenom(denom string) (*IBCTotalEscrowResponse, error) {
	request := fmt.Sprintf(`{"denom":"%s"}`, denom)

	resp, err := c.Invoke(methodTotalEscrowDenom, []byte(request))
	if err != nil {
		return nil, fmt.Errorf("GetIBCTotalEscrowForDenom(%s): %w", denom, err)
	}

	var result IBCTotalEscrowResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetIBCTotalEscrowForDenom: parse response: %w", err)
	}

	return &result, nil
}

// GetIBCTransferParams fetches IBC transfer module parameters.
func (c *Client) GetIBCTransferParams() (*IBCTransferParamsResponse, error) {
	resp, err := c.Invoke(methodIBCTransferParams, []byte("{}"))
	if err != nil {
		return nil, fmt.Errorf("GetIBCTransferParams: %w", err)
	}

	var result IBCTransferParamsResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("GetIBCTransferParams: parse response: %w", err)
	}

	return &result, nil
}
