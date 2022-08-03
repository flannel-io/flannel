// Copyright (c) 2017-2018 THL A29 Limited, a Tencent company. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v20170312

const (
	// 此产品的特有错误码

	// 账户配额不足，每个腾讯云账户每个地域下最多可创建 20 个 EIP。
	ADDRESSQUOTALIMITEXCEEDED = "AddressQuotaLimitExceeded"

	// 申购次数不足，每个腾讯云账户每个地域每天申购次数为配额数*2 次。
	ADDRESSQUOTALIMITEXCEEDED_DAILYALLOCATE = "AddressQuotaLimitExceeded.DailyAllocate"

	// CAM签名/鉴权错误。
	AUTHFAILURE = "AuthFailure"

	// 地址没有弹性网卡信息。
	FAILEDOPERATION_ADDRESSENIINFONOTFOUND = "FailedOperation.AddressEniInfoNotFound"

	// 账户余额不足。
	FAILEDOPERATION_BALANCEINSUFFICIENT = "FailedOperation.BalanceInsufficient"

	// 不支持的地域。
	FAILEDOPERATION_INVALIDREGION = "FailedOperation.InvalidRegion"

	// 未找到实例的主网卡。
	FAILEDOPERATION_MASTERENINOTFOUND = "FailedOperation.MasterEniNotFound"

	// 网络探测超时，请稍后重试。
	FAILEDOPERATION_NETDETECTTIMEOUT = "FailedOperation.NetDetectTimeOut"

	// 任务执行失败。
	FAILEDOPERATION_TASKFAILED = "FailedOperation.TaskFailed"

	// 内部错误。
	INTERNALERROR = "InternalError"

	// 创建Ckafka路由失败，请稍后重试。
	INTERNALERROR_CREATECKAFKAROUTEERROR = "InternalError.CreateCkafkaRouteError"

	// 操作内部错误。
	INTERNALSERVERERROR = "InternalServerError"

	// 不支持此账户。
	INVALIDACCOUNT_NOTSUPPORTED = "InvalidAccount.NotSupported"

	// 指定EIP处于被封堵状态。当EIP处于封堵状态的时候是不能够进行绑定操作的，需要先进行解封。
	INVALIDADDRESSID_BLOCKED = "InvalidAddressId.Blocked"

	//  指定的EIP不存在。
	INVALIDADDRESSID_NOTFOUND = "InvalidAddressId.NotFound"

	// 指定EIP处于欠费状态。
	INVALIDADDRESSIDSTATE_INARREARS = "InvalidAddressIdState.InArrears"

	// 指定 EIP 当前状态不能进行绑定操作。只有 EIP 的状态是 UNBIND 时才能进行绑定操作。
	INVALIDADDRESSIDSTATUS_NOTPERMIT = "InvalidAddressIdStatus.NotPermit"

	// 指定EIP的当前状态不允许进行该操作。
	INVALIDADDRESSSTATE = "InvalidAddressState"

	// 不被支持的实例。
	INVALIDINSTANCE_NOTSUPPORTED = "InvalidInstance.NotSupported"

	// 指定实例已经绑定了EIP。需先解绑当前的EIP才能再次进行绑定操作。
	INVALIDINSTANCEID_ALREADYBINDEIP = "InvalidInstanceId.AlreadyBindEip"

	// 无效实例ID。指定的实例ID不存在。
	INVALIDINSTANCEID_NOTFOUND = "InvalidInstanceId.NotFound"

	// 指定 NetworkInterfaceId 不存在或指定的PrivateIpAddress不在NetworkInterfaceId上。
	INVALIDNETWORKINTERFACEID_NOTFOUND = "InvalidNetworkInterfaceId.NotFound"

	// 参数错误。
	INVALIDPARAMETER = "InvalidParameter"

	// 参数不支持同时指定。
	INVALIDPARAMETER_COEXIST = "InvalidParameter.Coexist"

	// 指定过滤条件不存在。
	INVALIDPARAMETER_FILTERINVALIDKEY = "InvalidParameter.FilterInvalidKey"

	// 指定过滤条件不是键值对。
	INVALIDPARAMETER_FILTERNOTDICT = "InvalidParameter.FilterNotDict"

	// 指定过滤选项值不是列表。
	INVALIDPARAMETER_FILTERVALUESNOTLIST = "InvalidParameter.FilterValuesNotList"

	// 该过滤规则不合法。
	INVALIDPARAMETER_INVALIDFILTER = "InvalidParameter.InvalidFilter"

	// 下一跳类型与下一跳网关不匹配。
	INVALIDPARAMETER_NEXTHOPMISMATCH = "InvalidParameter.NextHopMismatch"

	// 专线网关跨可用区容灾组不存在。
	INVALIDPARAMETER_VPGHAGROUPNOTFOUND = "InvalidParameter.VpgHaGroupNotFound"

	// 指定的两个参数冲突，不能同时存在。 EIP只能绑定在实例上或指定网卡的指定内网 IP 上。
	INVALIDPARAMETERCONFLICT = "InvalidParameterConflict"

	// 参数取值错误。
	INVALIDPARAMETERVALUE = "InvalidParameterValue"

	// 被攻击的IP地址。
	INVALIDPARAMETERVALUE_ADDRESSATTACKED = "InvalidParameterValue.AddressAttacked"

	// 该地址ID不合法。
	INVALIDPARAMETERVALUE_ADDRESSIDMALFORMED = "InvalidParameterValue.AddressIdMalformed"

	// 该地址计费方式与其他地址冲突。
	INVALIDPARAMETERVALUE_ADDRESSINTERNETCHARGETYPECONFLICT = "InvalidParameterValue.AddressInternetChargeTypeConflict"

	// 该IP地址现在不可用。
	INVALIDPARAMETERVALUE_ADDRESSIPNOTAVAILABLE = "InvalidParameterValue.AddressIpNotAvailable"

	// IP地址未找到。
	INVALIDPARAMETERVALUE_ADDRESSIPNOTFOUND = "InvalidParameterValue.AddressIpNotFound"

	// VPC中不存在此IP地址。
	INVALIDPARAMETERVALUE_ADDRESSIPNOTINVPC = "InvalidParameterValue.AddressIpNotInVpc"

	// 此IPv6地址未发布。
	INVALIDPARAMETERVALUE_ADDRESSIPNOTPUBLIC = "InvalidParameterValue.AddressIpNotPublic"

	// 该地址不可与此实例申请。
	INVALIDPARAMETERVALUE_ADDRESSNOTAPPLICABLE = "InvalidParameterValue.AddressNotApplicable"

	// 该地址不是CalcIP。
	INVALIDPARAMETERVALUE_ADDRESSNOTCALCIP = "InvalidParameterValue.AddressNotCalcIP"

	// 该地址不是EIP。
	INVALIDPARAMETERVALUE_ADDRESSNOTEIP = "InvalidParameterValue.AddressNotEIP"

	// 未找到该地址。
	INVALIDPARAMETERVALUE_ADDRESSNOTFOUND = "InvalidParameterValue.AddressNotFound"

	// 该IPv6地址已经发布。
	INVALIDPARAMETERVALUE_ADDRESSPUBLISHED = "InvalidParameterValue.AddressPublished"

	// 带宽超出限制。
	INVALIDPARAMETERVALUE_BANDWIDTHOUTOFRANGE = "InvalidParameterValue.BandwidthOutOfRange"

	// 带宽包ID不正确。
	INVALIDPARAMETERVALUE_BANDWIDTHPACKAGEIDMALFORMED = "InvalidParameterValue.BandwidthPackageIdMalformed"

	// 该带宽包正在被使用。
	INVALIDPARAMETERVALUE_BANDWIDTHPACKAGEINUSE = "InvalidParameterValue.BandwidthPackageInUse"

	// 未查询到该带宽包。
	INVALIDPARAMETERVALUE_BANDWIDTHPACKAGENOTFOUND = "InvalidParameterValue.BandwidthPackageNotFound"

	// 选择带宽低于可允许的最小范围。
	INVALIDPARAMETERVALUE_BANDWIDTHTOOSMALL = "InvalidParameterValue.BandwidthTooSmall"

	// 指定云联网关联黑石私有网络数量达到上限。
	INVALIDPARAMETERVALUE_CCNATTACHBMVPCLIMITEXCEEDED = "InvalidParameterValue.CcnAttachBmvpcLimitExceeded"

	// 目的网段不在对端VPC的CIDR范围内。
	INVALIDPARAMETERVALUE_CIDRNOTINPEERVPC = "InvalidParameterValue.CidrNotInPeerVpc"

	// 指定CIDR不在SSL-VPN所属私有网络CIDR内。
	INVALIDPARAMETERVALUE_CIDRNOTINSSLVPNVPC = "InvalidParameterValue.CidrNotInSslVpnVpc"

	// 非法入参组合。
	INVALIDPARAMETERVALUE_COMBINATION = "InvalidParameterValue.Combination"

	// 入参重复。
	INVALIDPARAMETERVALUE_DUPLICATE = "InvalidParameterValue.Duplicate"

	// 参数值存在重复。
	INVALIDPARAMETERVALUE_DUPLICATEPARA = "InvalidParameterValue.DuplicatePara"

	// 值超过上限。
	INVALIDPARAMETERVALUE_EIPBRANDWIDTHOUTINVALID = "InvalidParameterValue.EIPBrandWidthOutInvalid"

	// 缺少参数。
	INVALIDPARAMETERVALUE_EMPTY = "InvalidParameterValue.Empty"

	// IPv6规则没有更改。
	INVALIDPARAMETERVALUE_IPV6RULENOTCHANGE = "InvalidParameterValue.IPv6RuleNotChange"

	// 该实例的计费方式与其他实例不同。
	INVALIDPARAMETERVALUE_INCONSISTENTINSTANCEINTERNETCHARGETYPE = "InvalidParameterValue.InconsistentInstanceInternetChargeType"

	// 该实例不支持AnycastEIP。
	INVALIDPARAMETERVALUE_INSTANCEDOESNOTSUPPORTANYCAST = "InvalidParameterValue.InstanceDoesNotSupportAnycast"

	// 实例不存在公网IP。
	INVALIDPARAMETERVALUE_INSTANCEHASNOWANIP = "InvalidParameterValue.InstanceHasNoWanIP"

	// 该实例已有WanIP。
	INVALIDPARAMETERVALUE_INSTANCEHASWANIP = "InvalidParameterValue.InstanceHasWanIP"

	// 实例ID错误。
	INVALIDPARAMETERVALUE_INSTANCEIDMALFORMED = "InvalidParameterValue.InstanceIdMalformed"

	// 该实例没有CalcIP，无法完成请求。
	INVALIDPARAMETERVALUE_INSTANCENOCALCIP = "InvalidParameterValue.InstanceNoCalcIP"

	// 该实例没有WanIP，无法完成请求。
	INVALIDPARAMETERVALUE_INSTANCENOWANIP = "InvalidParameterValue.InstanceNoWanIP"

	// 由于该IP被禁用，无法绑定该实例。
	INVALIDPARAMETERVALUE_INSTANCENORMALPUBLICIPBLOCKED = "InvalidParameterValue.InstanceNormalPublicIpBlocked"

	// 弹性网卡绑定的实例与地址绑定的实例不一致。
	INVALIDPARAMETERVALUE_INSTANCENOTMATCHASSOCIATEENI = "InvalidParameterValue.InstanceNotMatchAssociateEni"

	// 网络计费模式没有更改。
	INVALIDPARAMETERVALUE_INTERNETCHARGETYPENOTCHANGED = "InvalidParameterValue.InternetChargeTypeNotChanged"

	// 无效的带宽包计费方式。
	INVALIDPARAMETERVALUE_INVALIDBANDWIDTHPACKAGECHARGETYPE = "InvalidParameterValue.InvalidBandwidthPackageChargeType"

	// 参数的值不存在或不支持。
	INVALIDPARAMETERVALUE_INVALIDBUSINESS = "InvalidParameterValue.InvalidBusiness"

	// 传入的DedicatedClusterId有误。
	INVALIDPARAMETERVALUE_INVALIDDEDICATEDCLUSTERID = "InvalidParameterValue.InvalidDedicatedClusterId"

	// 该IP只能绑定小时流量后付费和带宽包实例。
	INVALIDPARAMETERVALUE_INVALIDINSTANCEINTERNETCHARGETYPE = "InvalidParameterValue.InvalidInstanceInternetChargeType"

	// 该实例状态无法完成操作。
	INVALIDPARAMETERVALUE_INVALIDINSTANCESTATE = "InvalidParameterValue.InvalidInstanceState"

	// 无效的IPv6地址。
	INVALIDPARAMETERVALUE_INVALIDIPV6 = "InvalidParameterValue.InvalidIpv6"

	// 该Tag不合法。
	INVALIDPARAMETERVALUE_INVALIDTAG = "InvalidParameterValue.InvalidTag"

	// 负载均衡实例已经绑定了EIP。
	INVALIDPARAMETERVALUE_LBALREADYBINDEIP = "InvalidParameterValue.LBAlreadyBindEip"

	// 参数值超出限制。
	INVALIDPARAMETERVALUE_LIMITEXCEEDED = "InvalidParameterValue.LimitExceeded"

	// 入参格式不合法。
	INVALIDPARAMETERVALUE_MALFORMED = "InvalidParameterValue.Malformed"

	// 缺少绑定的实例。
	INVALIDPARAMETERVALUE_MISSINGASSOCIATEENTITY = "InvalidParameterValue.MissingAssociateEntity"

	// 集群类型不同的IP不可在同一请求中。
	INVALIDPARAMETERVALUE_MIXEDADDRESSIPSETTYPE = "InvalidParameterValue.MixedAddressIpSetType"

	// NAT网关的SNAT转换规则不存在。
	INVALIDPARAMETERVALUE_NATGATEWAYSNATRULENOTEXISTS = "InvalidParameterValue.NatGatewaySnatRuleNotExists"

	// NAT网关的SNAT规则已经存在。
	INVALIDPARAMETERVALUE_NATSNATRULEEXISTS = "InvalidParameterValue.NatSnatRuleExists"

	// 探测目的IP和网络探测在同一个VPC内。
	INVALIDPARAMETERVALUE_NETDETECTINVPC = "InvalidParameterValue.NetDetectInVpc"

	// 探测目的IP在云联网的路由表中找不到匹配的下一跳。
	INVALIDPARAMETERVALUE_NETDETECTNOTFOUNDIP = "InvalidParameterValue.NetDetectNotFoundIp"

	// 探测目的IP与同一个私有网络内的同一个子网下的其他网络探测的探测目的IP相同。
	INVALIDPARAMETERVALUE_NETDETECTSAMEIP = "InvalidParameterValue.NetDetectSameIp"

	// 网络接口ID不正确。
	INVALIDPARAMETERVALUE_NETWORKINTERFACEIDMALFORMED = "InvalidParameterValue.NetworkInterfaceIdMalformed"

	// 未找到网络接口ID，或私有IP地址未在网络接口配置。
	INVALIDPARAMETERVALUE_NETWORKINTERFACENOTFOUND = "InvalidParameterValue.NetworkInterfaceNotFound"

	// 该操作仅对主网卡支持。
	INVALIDPARAMETERVALUE_ONLYSUPPORTEDFORMASTERNETWORKCARD = "InvalidParameterValue.OnlySupportedForMasterNetworkCard"

	// 参数值不在指定范围。
	INVALIDPARAMETERVALUE_RANGE = "InvalidParameterValue.Range"

	// 参数值是一个系统保留对象。
	INVALIDPARAMETERVALUE_RESERVED = "InvalidParameterValue.Reserved"

	// 该资源已加入其他带宽包。
	INVALIDPARAMETERVALUE_RESOURCEALREADYEXISTED = "InvalidParameterValue.ResourceAlreadyExisted"

	// 该资源已过期。
	INVALIDPARAMETERVALUE_RESOURCEEXPIRED = "InvalidParameterValue.ResourceExpired"

	// 资源ID不正确。
	INVALIDPARAMETERVALUE_RESOURCEIDMALFORMED = "InvalidParameterValue.ResourceIdMalformed"

	// 该资源不在此带宽包中。
	INVALIDPARAMETERVALUE_RESOURCENOTEXISTED = "InvalidParameterValue.ResourceNotExisted"

	// 未查询到该资源。
	INVALIDPARAMETERVALUE_RESOURCENOTFOUND = "InvalidParameterValue.ResourceNotFound"

	// 该资源不支持此操作。
	INVALIDPARAMETERVALUE_RESOURCENOTSUPPORT = "InvalidParameterValue.ResourceNotSupport"

	// 子网CIDR冲突。
	INVALIDPARAMETERVALUE_SUBNETCONFLICT = "InvalidParameterValue.SubnetConflict"

	// CIDR与同一个私有网络内的另一个子网发生重叠。
	INVALIDPARAMETERVALUE_SUBNETOVERLAP = "InvalidParameterValue.SubnetOverlap"

	// 子网与辅助Cidr网段重叠。
	INVALIDPARAMETERVALUE_SUBNETOVERLAPASSISTCIDR = "InvalidParameterValue.SubnetOverlapAssistCidr"

	// 子网CIDR不合法。
	INVALIDPARAMETERVALUE_SUBNETRANGE = "InvalidParameterValue.SubnetRange"

	// 标签键重复。
	INVALIDPARAMETERVALUE_TAGDUPLICATEKEY = "InvalidParameterValue.TagDuplicateKey"

	// 重复的标签资源类型。
	INVALIDPARAMETERVALUE_TAGDUPLICATERESOURCETYPE = "InvalidParameterValue.TagDuplicateResourceType"

	// 标签键无效。
	INVALIDPARAMETERVALUE_TAGINVALIDKEY = "InvalidParameterValue.TagInvalidKey"

	// 标签键长度无效。
	INVALIDPARAMETERVALUE_TAGINVALIDKEYLEN = "InvalidParameterValue.TagInvalidKeyLen"

	// 标签值无效。
	INVALIDPARAMETERVALUE_TAGINVALIDVAL = "InvalidParameterValue.TagInvalidVal"

	// 标签键不存在。
	INVALIDPARAMETERVALUE_TAGKEYNOTEXISTS = "InvalidParameterValue.TagKeyNotExists"

	// 标签没有分配配额。
	INVALIDPARAMETERVALUE_TAGNOTALLOCATEDQUOTA = "InvalidParameterValue.TagNotAllocatedQuota"

	// 该标签和值不存在。
	INVALIDPARAMETERVALUE_TAGNOTEXISTED = "InvalidParameterValue.TagNotExisted"

	// 不支持的标签。
	INVALIDPARAMETERVALUE_TAGNOTSUPPORTTAG = "InvalidParameterValue.TagNotSupportTag"

	// '标签资源格式错误。
	INVALIDPARAMETERVALUE_TAGRESOURCEFORMATERROR = "InvalidParameterValue.TagResourceFormatError"

	// 标签时间戳超配。
	INVALIDPARAMETERVALUE_TAGTIMESTAMPEXCEEDED = "InvalidParameterValue.TagTimestampExceeded"

	// 标签值不存在。
	INVALIDPARAMETERVALUE_TAGVALNOTEXISTS = "InvalidParameterValue.TagValNotExists"

	// 无效参数值。参数值太长。
	INVALIDPARAMETERVALUE_TOOLONG = "InvalidParameterValue.TooLong"

	// 该可用区不可用。
	INVALIDPARAMETERVALUE_UNAVAILABLEZONE = "InvalidParameterValue.UnavailableZone"

	// 目的网段和当前VPC的CIDR冲突。
	INVALIDPARAMETERVALUE_VPCCIDRCONFLICT = "InvalidParameterValue.VpcCidrConflict"

	// 当前功能不支持此专线网关。
	INVALIDPARAMETERVALUE_VPGTYPENOTMATCH = "InvalidParameterValue.VpgTypeNotMatch"

	// 目的网段和当前VPN通道的CIDR冲突。
	INVALIDPARAMETERVALUE_VPNCONNCIDRCONFLICT = "InvalidParameterValue.VpnConnCidrConflict"

	// VPN通道探测ip冲突。
	INVALIDPARAMETERVALUE_VPNCONNHEALTHCHECKIPCONFLICT = "InvalidParameterValue.VpnConnHealthCheckIpConflict"

	// 参数Zone的值与CDC所在Zone冲突。
	INVALIDPARAMETERVALUE_ZONECONFLICT = "InvalidParameterValue.ZoneConflict"

	// 指定弹性网卡的指定内网IP已经绑定了EIP，不能重复绑定。
	INVALIDPRIVATEIPADDRESS_ALREADYBINDEIP = "InvalidPrivateIpAddress.AlreadyBindEip"

	// 无效的路由策略ID（RouteId）。
	INVALIDROUTEID_NOTFOUND = "InvalidRouteId.NotFound"

	// 无效的路由表,路由表实例ID不合法。
	INVALIDROUTETABLEID_MALFORMED = "InvalidRouteTableId.Malformed"

	// 无效的路由表,路由表资源不存在，请再次核实您输入的资源信息是否正确。
	INVALIDROUTETABLEID_NOTFOUND = "InvalidRouteTableId.NotFound"

	// 无效的安全组,安全组实例ID不合法。
	INVALIDSECURITYGROUPID_MALFORMED = "InvalidSecurityGroupID.Malformed"

	// 无效的安全组,安全组实例ID不存在。
	INVALIDSECURITYGROUPID_NOTFOUND = "InvalidSecurityGroupID.NotFound"

	// 无效的VPC,VPC实例ID不合法。
	INVALIDVPCID_MALFORMED = "InvalidVpcId.Malformed"

	// 无效的VPC,VPC资源不存在。
	INVALIDVPCID_NOTFOUND = "InvalidVpcId.NotFound"

	// 无效的VPN网关,VPN实例ID不合法。
	INVALIDVPNGATEWAYID_MALFORMED = "InvalidVpnGatewayId.Malformed"

	// 无效的VPN网关,VPN实例不存在，请再次核实您输入的资源信息是否正确。
	INVALIDVPNGATEWAYID_NOTFOUND = "InvalidVpnGatewayId.NotFound"

	// 超过配额限制。
	LIMITEXCEEDED = "LimitExceeded"

	// 账号退还配额超过限制。
	LIMITEXCEEDED_ACCOUNTRETURNQUOTA = "LimitExceeded.AccountReturnQuota"

	// 分配IP地址数量达到上限。
	LIMITEXCEEDED_ADDRESS = "LimitExceeded.Address"

	// 租户申请的弹性IP超过上限。
	LIMITEXCEEDED_ADDRESSQUOTALIMITEXCEEDED = "LimitExceeded.AddressQuotaLimitExceeded"

	// 带宽包配额超过限制。
	LIMITEXCEEDED_BANDWIDTHPACKAGEQUOTA = "LimitExceeded.BandwidthPackageQuota"

	// 超过更换IP配额。
	LIMITEXCEEDED_CHANGEADDRESSQUOTA = "LimitExceeded.ChangeAddressQuota"

	// VPC分配网段数量达到上限。
	LIMITEXCEEDED_CIDRBLOCK = "LimitExceeded.CidrBlock"

	// 租户每天申请的弹性IP超过上限。
	LIMITEXCEEDED_DAILYALLOCATEADDRESSQUOTALIMITEXCEEDED = "LimitExceeded.DailyAllocateAddressQuotaLimitExceeded"

	// 超过每日更换IP配额。
	LIMITEXCEEDED_DAILYCHANGEADDRESSQUOTA = "LimitExceeded.DailyChangeAddressQuota"

	// 实例绑定的弹性IP超过配额。
	LIMITEXCEEDED_INSTANCEADDRESSQUOTA = "LimitExceeded.InstanceAddressQuota"

	// 修改地址网络计费模式配额超过限制。
	LIMITEXCEEDED_MODIFYADDRESSINTERNETCHARGETYPEQUOTA = "LimitExceeded.ModifyAddressInternetChargeTypeQuota"

	// 每月地址找回配额超过限制。
	LIMITEXCEEDED_MONTHLYADDRESSRECOVERYQUOTA = "LimitExceeded.MonthlyAddressRecoveryQuota"

	// NAT网关数量已达到上限。
	LIMITEXCEEDED_NATGATEWAYLIMITEXCEEDED = "LimitExceeded.NatGatewayLimitExceeded"

	// 私有网络创建的NAT网关超过上限。
	LIMITEXCEEDED_NATGATEWAYPERVPCLIMITEXCEEDED = "LimitExceeded.NatGatewayPerVpcLimitExceeded"

	// 过滤参数名称超过限制。
	LIMITEXCEEDED_NUMBEROFFILTERS = "LimitExceeded.NumberOfFilters"

	// NAT网关绑定的弹性IP超过上限。
	LIMITEXCEEDED_PUBLICIPADDRESSPERNATGATEWAYLIMITEXCEEDED = "LimitExceeded.PublicIpAddressPerNatGatewayLimitExceeded"

	// 安全组规则数量超过上限。
	LIMITEXCEEDED_SECURITYGROUPPOLICYSET = "LimitExceeded.SecurityGroupPolicySet"

	// 子网分配子网段数量达到上限。
	LIMITEXCEEDED_SUBNETCIDRBLOCK = "LimitExceeded.SubnetCidrBlock"

	// 标签键已达到上限。
	LIMITEXCEEDED_TAGKEYEXCEEDED = "LimitExceeded.TagKeyExceeded"

	// 每个资源的标签键已达到上限。
	LIMITEXCEEDED_TAGKEYPERRESOURCEEXCEEDED = "LimitExceeded.TagKeyPerResourceExceeded"

	// 没有足够的标签配额。
	LIMITEXCEEDED_TAGNOTENOUGHQUOTA = "LimitExceeded.TagNotEnoughQuota"

	// 标签配额已满，无法创建资源。
	LIMITEXCEEDED_TAGQUOTA = "LimitExceeded.TagQuota"

	// 标签配额已达到上限。
	LIMITEXCEEDED_TAGQUOTAEXCEEDED = "LimitExceeded.TagQuotaExceeded"

	// 标签键的数目已达到上限。
	LIMITEXCEEDED_TAGTAGSEXCEEDED = "LimitExceeded.TagTagsExceeded"

	// 缺少参数错误。
	MISSINGPARAMETER = "MissingParameter"

	// 指定公网IP处于隔离状态。
	OPERATIONDENIED_ADDRESSINARREARS = "OperationDenied.AddressInArrears"

	// 互斥的任务正在执行。
	OPERATIONDENIED_MUTEXTASKRUNNING = "OperationDenied.MutexTaskRunning"

	// 资源被占用。
	RESOURCEINUSE = "ResourceInUse"

	// 指定IP地址已经在使用中。
	RESOURCEINUSE_ADDRESS = "ResourceInUse.Address"

	// 资源不足。
	RESOURCEINSUFFICIENT = "ResourceInsufficient"

	// 网段资源不足。
	RESOURCEINSUFFICIENT_CIDRBLOCK = "ResourceInsufficient.CidrBlock"

	// 资源不存在。
	RESOURCENOTFOUND = "ResourceNotFound"

	// Svc不存在。
	RESOURCENOTFOUND_SVCNOTEXIST = "ResourceNotFound.SvcNotExist"

	// 资源不可用。
	RESOURCEUNAVAILABLE = "ResourceUnavailable"

	// 当前用户不在指定终端节点服务的白名单内。
	RESOURCEUNAVAILABLE_SERVICEWHITELISTNOTADDED = "ResourceUnavailable.ServiceWhiteListNotAdded"

	// 未授权操作。
	UNAUTHORIZEDOPERATION = "UnauthorizedOperation"

	// 无权限申请AnycastEip资源。
	UNAUTHORIZEDOPERATION_ANYCASTEIP = "UnauthorizedOperation.AnycastEip"

	// 绑定关系不存在。
	UNAUTHORIZEDOPERATION_ATTACHMENTNOTFOUND = "UnauthorizedOperation.AttachmentNotFound"

	// 未授权的用户。
	UNAUTHORIZEDOPERATION_INVALIDACCOUNT = "UnauthorizedOperation.InvalidAccount"

	// 账号未实名。
	UNAUTHORIZEDOPERATION_NOREALNAMEAUTHENTICATION = "UnauthorizedOperation.NoRealNameAuthentication"

	// 主IP不支持指定操作。
	UNAUTHORIZEDOPERATION_PRIMARYIP = "UnauthorizedOperation.PrimaryIp"

	// 未知参数错误。
	UNKNOWNPARAMETER = "UnknownParameter"

	// 参数无法识别，可以尝试相似参数代替。
	UNKNOWNPARAMETER_WITHGUESS = "UnknownParameter.WithGuess"

	// 操作不支持。
	UNSUPPORTEDOPERATION = "UnsupportedOperation"

	// 接口不存在。
	UNSUPPORTEDOPERATION_ACTIONNOTFOUND = "UnsupportedOperation.ActionNotFound"

	// 欠费状态不支持该操作。
	UNSUPPORTEDOPERATION_ADDRESSIPINARREAR = "UnsupportedOperation.AddressIpInArrear"

	// 此付费模式的IP地址不支持该操作。
	UNSUPPORTEDOPERATION_ADDRESSIPINTERNETCHARGETYPENOTPERMIT = "UnsupportedOperation.AddressIpInternetChargeTypeNotPermit"

	// 绑定此实例的IP地址不支持该操作。
	UNSUPPORTEDOPERATION_ADDRESSIPNOTSUPPORTINSTANCE = "UnsupportedOperation.AddressIpNotSupportInstance"

	// 此IP地址状态不支持该操作。
	UNSUPPORTEDOPERATION_ADDRESSIPSTATUSNOTPERMIT = "UnsupportedOperation.AddressIpStatusNotPermit"

	// 该地址状态不支持此操作。
	UNSUPPORTEDOPERATION_ADDRESSSTATUSNOTPERMIT = "UnsupportedOperation.AddressStatusNotPermit"

	// 资源不在指定的AppId下。
	UNSUPPORTEDOPERATION_APPIDMISMATCH = "UnsupportedOperation.AppIdMismatch"

	// APPId不存在。
	UNSUPPORTEDOPERATION_APPIDNOTFOUND = "UnsupportedOperation.AppIdNotFound"

	// 绑定关系已存在。
	UNSUPPORTEDOPERATION_ATTACHMENTALREADYEXISTS = "UnsupportedOperation.AttachmentAlreadyExists"

	// 绑定关系不存在。
	UNSUPPORTEDOPERATION_ATTACHMENTNOTFOUND = "UnsupportedOperation.AttachmentNotFound"

	// 当前云联网还有预付费带宽未到期，不支持主动删除。
	UNSUPPORTEDOPERATION_BANDWIDTHNOTEXPIRED = "UnsupportedOperation.BandwidthNotExpired"

	// 该带宽包不支持此操作。
	UNSUPPORTEDOPERATION_BANDWIDTHPACKAGEIDNOTSUPPORTED = "UnsupportedOperation.BandwidthPackageIdNotSupported"

	// 已绑定EIP。
	UNSUPPORTEDOPERATION_BINDEIP = "UnsupportedOperation.BindEIP"

	// 指定VPC CIDR范围不支持私有网络和基础网络设备互通。
	UNSUPPORTEDOPERATION_CIDRUNSUPPORTEDCLASSICLINK = "UnsupportedOperation.CIDRUnSupportedClassicLink"

	// 实例已关联CCN。
	UNSUPPORTEDOPERATION_CCNATTACHED = "UnsupportedOperation.CcnAttached"

	// 当前云联网有流日志，不支持删除。
	UNSUPPORTEDOPERATION_CCNHASFLOWLOG = "UnsupportedOperation.CcnHasFlowLog"

	// 实例未关联CCN。
	UNSUPPORTEDOPERATION_CCNNOTATTACHED = "UnsupportedOperation.CcnNotAttached"

	// 跨账号场景下不支持自驾云账号实例 关联普通账号云联网。
	UNSUPPORTEDOPERATION_CCNORDINARYACCOUNTREFUSEATTACH = "UnsupportedOperation.CcnOrdinaryAccountRefuseAttach"

	// 指定的路由表不存在。
	UNSUPPORTEDOPERATION_CCNROUTETABLENOTEXIST = "UnsupportedOperation.CcnRouteTableNotExist"

	// CDC子网不支持创建非本地网关类型的路由。
	UNSUPPORTEDOPERATION_CDCSUBNETNOTSUPPORTUNLOCALGATEWAY = "UnsupportedOperation.CdcSubnetNotSupportUnLocalGateway"

	// 实例已经和VPC绑定。
	UNSUPPORTEDOPERATION_CLASSICINSTANCEIDALREADYEXISTS = "UnsupportedOperation.ClassicInstanceIdAlreadyExists"

	// 公网Clb不支持该规则。
	UNSUPPORTEDOPERATION_CLBPOLICYLIMIT = "UnsupportedOperation.ClbPolicyLimit"

	// 与该VPC下的TKE容器的网段重叠。
	UNSUPPORTEDOPERATION_CONFLICTWITHDOCKERROUTE = "UnsupportedOperation.ConflictWithDockerRoute"

	// 该专线网关存在关联的NAT规则，不允许删除，请先删调所有的NAT规则。
	UNSUPPORTEDOPERATION_DCGATEWAYNATRULEEXISTS = "UnsupportedOperation.DCGatewayNatRuleExists"

	// 指定的VPC未发现专线网关。
	UNSUPPORTEDOPERATION_DCGATEWAYSNOTFOUNDINVPC = "UnsupportedOperation.DcGatewaysNotFoundInVpc"

	// 禁止删除默认路由表。
	UNSUPPORTEDOPERATION_DELDEFAULTROUTE = "UnsupportedOperation.DelDefaultRoute"

	// 禁止删除已关联子网的路由表。
	UNSUPPORTEDOPERATION_DELROUTEWITHSUBNET = "UnsupportedOperation.DelRouteWithSubnet"

	// 专线网关正在更新BGP Community属性。
	UNSUPPORTEDOPERATION_DIRECTCONNECTGATEWAYISUPDATINGCOMMUNITY = "UnsupportedOperation.DirectConnectGatewayIsUpdatingCommunity"

	// 指定的路由策略已发布至云联网，请先撤销。
	UNSUPPORTEDOPERATION_DISABLEDNOTIFYCCN = "UnsupportedOperation.DisabledNotifyCcn"

	// 安全组规则重复。
	UNSUPPORTEDOPERATION_DUPLICATEPOLICY = "UnsupportedOperation.DuplicatePolicy"

	// 不支持ECMP。
	UNSUPPORTEDOPERATION_ECMP = "UnsupportedOperation.Ecmp"

	// 和云联网的路由形成ECMP。
	UNSUPPORTEDOPERATION_ECMPWITHCCNROUTE = "UnsupportedOperation.EcmpWithCcnRoute"

	// 和用户自定义的路由形成ECMP。
	UNSUPPORTEDOPERATION_ECMPWITHUSERROUTE = "UnsupportedOperation.EcmpWithUserRoute"

	// 终端节点服务本身不能是终端节点。
	UNSUPPORTEDOPERATION_ENDPOINTSERVICE = "UnsupportedOperation.EndPointService"

	// 不支持创建流日志：当前弹性网卡绑定的是KO机型。
	UNSUPPORTEDOPERATION_FLOWLOGSNOTSUPPORTKOINSTANCEENI = "UnsupportedOperation.FlowLogsNotSupportKoInstanceEni"

	// 不支持创建流日志：当前弹性网卡未绑定实例。
	UNSUPPORTEDOPERATION_FLOWLOGSNOTSUPPORTNULLINSTANCEENI = "UnsupportedOperation.FlowLogsNotSupportNullInstanceEni"

	// 该种类型地址不支持此操作。
	UNSUPPORTEDOPERATION_INCORRECTADDRESSRESOURCETYPE = "UnsupportedOperation.IncorrectAddressResourceType"

	// 用户配置的实例和路由表不匹配。
	UNSUPPORTEDOPERATION_INSTANCEANDRTBNOTMATCH = "UnsupportedOperation.InstanceAndRtbNotMatch"

	// 指定实例资源不匹配。
	UNSUPPORTEDOPERATION_INSTANCEMISMATCH = "UnsupportedOperation.InstanceMismatch"

	// 跨账号场景下不支持普通账号实例关联自驾云账号云联网。
	UNSUPPORTEDOPERATION_INSTANCEORDINARYACCOUNTREFUSEATTACH = "UnsupportedOperation.InstanceOrdinaryAccountRefuseAttach"

	// 该地址绑定的实例状态不支持此操作。
	UNSUPPORTEDOPERATION_INSTANCESTATENOTSUPPORTED = "UnsupportedOperation.InstanceStateNotSupported"

	// 账户余额不足。
	UNSUPPORTEDOPERATION_INSUFFICIENTFUNDS = "UnsupportedOperation.InsufficientFunds"

	// 不支持该操作。
	UNSUPPORTEDOPERATION_INVALIDACTION = "UnsupportedOperation.InvalidAction"

	// 该地址的网络付费方式不支持此操作。
	UNSUPPORTEDOPERATION_INVALIDADDRESSINTERNETCHARGETYPE = "UnsupportedOperation.InvalidAddressInternetChargeType"

	// 该地址状态不支持此操作。
	UNSUPPORTEDOPERATION_INVALIDADDRESSSTATE = "UnsupportedOperation.InvalidAddressState"

	// 无效的实例状态。
	UNSUPPORTEDOPERATION_INVALIDINSTANCESTATE = "UnsupportedOperation.InvalidInstanceState"

	// 该计费方式不支持此操作。
	UNSUPPORTEDOPERATION_INVALIDRESOURCEINTERNETCHARGETYPE = "UnsupportedOperation.InvalidResourceInternetChargeType"

	// 不支持加入此协议的带宽包。
	UNSUPPORTEDOPERATION_INVALIDRESOURCEPROTOCOL = "UnsupportedOperation.InvalidResourceProtocol"

	// 资源状态不合法。
	UNSUPPORTEDOPERATION_INVALIDSTATE = "UnsupportedOperation.InvalidState"

	// 当前状态不支持发布至云联网，请重试。
	UNSUPPORTEDOPERATION_INVALIDSTATUSNOTIFYCCN = "UnsupportedOperation.InvalidStatusNotifyCcn"

	// 关联当前云联网的实例的账号存在不是金融云账号。
	UNSUPPORTEDOPERATION_ISNOTFINANCEACCOUNT = "UnsupportedOperation.IsNotFinanceAccount"

	// 该ISP不支持此操作。
	UNSUPPORTEDOPERATION_ISPNOTSUPPORTED = "UnsupportedOperation.IspNotSupported"

	// 指定的CDC已存在本地网关。
	UNSUPPORTEDOPERATION_LOCALGATEWAYALREADYEXISTS = "UnsupportedOperation.LocalGatewayAlreadyExists"

	// 账户不支持修改公网IP的该属性。
	UNSUPPORTEDOPERATION_MODIFYADDRESSATTRIBUTE = "UnsupportedOperation.ModifyAddressAttribute"

	// 资源互斥操作任务正在执行。
	UNSUPPORTEDOPERATION_MUTEXOPERATIONTASKRUNNING = "UnsupportedOperation.MutexOperationTaskRunning"

	// SNAT/DNAT转换规则所指定的内网IP已绑定了其他的规则，无法重复绑定。
	UNSUPPORTEDOPERATION_NATGATEWAYRULEPIPEXISTS = "UnsupportedOperation.NatGatewayRulePipExists"

	// NAT网关类型不支持SNAT规则。
	UNSUPPORTEDOPERATION_NATGATEWAYTYPENOTSUPPORTSNAT = "UnsupportedOperation.NatGatewayTypeNotSupportSNAT"

	// NAT实例不支持该操作。
	UNSUPPORTEDOPERATION_NATNOTSUPPORTED = "UnsupportedOperation.NatNotSupported"

	// 指定的子网不支持创建本地网关类型的路由。
	UNSUPPORTEDOPERATION_NORMALSUBNETNOTSUPPORTLOCALGATEWAY = "UnsupportedOperation.NormalSubnetNotSupportLocalGateway"

	// 当前实例已被封禁，无法进行此操作。
	UNSUPPORTEDOPERATION_NOTLOCKEDINSTANCEOPERATION = "UnsupportedOperation.NotLockedInstanceOperation"

	// 当前云联网实例未处于申请中状态，无法进行操作。
	UNSUPPORTEDOPERATION_NOTPENDINGCCNINSTANCE = "UnsupportedOperation.NotPendingCcnInstance"

	// 当前云联网为非后付费类型，无法进行此操作。
	UNSUPPORTEDOPERATION_NOTPOSTPAIDCCNOPERATION = "UnsupportedOperation.NotPostpaidCcnOperation"

	// 不支持删除默认路由表。
	UNSUPPORTEDOPERATION_NOTSUPPORTDELETEDEFAULTROUTETABLE = "UnsupportedOperation.NotSupportDeleteDefaultRouteTable"

	// 当前云联网不支持更新路由发布。
	UNSUPPORTEDOPERATION_NOTSUPPORTEDUPDATECCNROUTEPUBLISH = "UnsupportedOperation.NotSupportedUpdateCcnRoutePublish"

	// 指定的路由策略不支持发布或撤销至云联网。
	UNSUPPORTEDOPERATION_NOTIFYCCN = "UnsupportedOperation.NotifyCcn"

	// 此产品计费方式已下线，请尝试其他计费方式。
	UNSUPPORTEDOPERATION_OFFLINECHARGETYPE = "UnsupportedOperation.OfflineChargeType"

	// 仅支持专业版Ckafka。
	UNSUPPORTEDOPERATION_ONLYSUPPORTPROFESSIONKAFKA = "UnsupportedOperation.OnlySupportProfessionKafka"

	// 预付费云联网只支持地域间限速。
	UNSUPPORTEDOPERATION_PREPAIDCCNONLYSUPPORTINTERREGIONLIMIT = "UnsupportedOperation.PrepaidCcnOnlySupportInterRegionLimit"

	// 指定的值是主IP。
	UNSUPPORTEDOPERATION_PRIMARYIP = "UnsupportedOperation.PrimaryIp"

	// Nat网关至少存在一个弹性IP，弹性IP不能解绑。
	UNSUPPORTEDOPERATION_PUBLICIPADDRESSDISASSOCIATE = "UnsupportedOperation.PublicIpAddressDisassociate"

	// 绑定NAT网关的弹性IP不是BGP性质的IP。
	UNSUPPORTEDOPERATION_PUBLICIPADDRESSISNOTBGPIP = "UnsupportedOperation.PublicIpAddressIsNotBGPIp"

	// 绑定NAT网关的弹性IP不存在。
	UNSUPPORTEDOPERATION_PUBLICIPADDRESSISNOTEXISTED = "UnsupportedOperation.PublicIpAddressIsNotExisted"

	// 绑定NAT网关的弹性IP不是按流量计费的。
	UNSUPPORTEDOPERATION_PUBLICIPADDRESSNOTBILLEDBYTRAFFIC = "UnsupportedOperation.PublicIpAddressNotBilledByTraffic"

	// 当前账号不能在该地域使用产品。
	UNSUPPORTEDOPERATION_PURCHASELIMIT = "UnsupportedOperation.PurchaseLimit"

	// 记录已存在。
	UNSUPPORTEDOPERATION_RECORDEXISTS = "UnsupportedOperation.RecordExists"

	// 记录不存在。
	UNSUPPORTEDOPERATION_RECORDNOTEXISTS = "UnsupportedOperation.RecordNotExists"

	// 输入的资源ID与IP绑定的资源不匹配，请检查。
	UNSUPPORTEDOPERATION_RESOURCEMISMATCH = "UnsupportedOperation.ResourceMismatch"

	// 未找到相关角色，请确认角色是否授权。
	UNSUPPORTEDOPERATION_ROLENOTFOUND = "UnsupportedOperation.RoleNotFound"

	// 路由表绑定了子网。
	UNSUPPORTEDOPERATION_ROUTETABLEHASSUBNETRULE = "UnsupportedOperation.RouteTableHasSubnetRule"

	// 指定的终端节点服务所创建的终端节点不支持绑定安全组。
	UNSUPPORTEDOPERATION_SPECIALENDPOINTSERVICE = "UnsupportedOperation.SpecialEndPointService"

	// SslVpnClientId 不存在。
	UNSUPPORTEDOPERATION_SSLVPNCLIENTIDNOTFOUND = "UnsupportedOperation.SslVpnClientIdNotFound"

	// 中继网卡不支持该操作。
	UNSUPPORTEDOPERATION_SUBENINOTSUPPORTTRUNKING = "UnsupportedOperation.SubEniNotSupportTrunking"

	// 系统路由，禁止操作。
	UNSUPPORTEDOPERATION_SYSTEMROUTE = "UnsupportedOperation.SystemRoute"

	// 标签正在分配中。
	UNSUPPORTEDOPERATION_TAGALLOCATE = "UnsupportedOperation.TagAllocate"

	// 标签正在释放中。
	UNSUPPORTEDOPERATION_TAGFREE = "UnsupportedOperation.TagFree"

	// 标签没有权限。
	UNSUPPORTEDOPERATION_TAGNOTPERMIT = "UnsupportedOperation.TagNotPermit"

	// 不支持使用系统预留的标签键。
	UNSUPPORTEDOPERATION_TAGSYSTEMRESERVEDTAGKEY = "UnsupportedOperation.TagSystemReservedTagKey"

	// 账号ID不存在。
	UNSUPPORTEDOPERATION_UINNOTFOUND = "UnsupportedOperation.UinNotFound"

	// 不支持跨境。
	UNSUPPORTEDOPERATION_UNABLECROSSBORDER = "UnsupportedOperation.UnableCrossBorder"

	// 当前云联网无法关联金融云实例。
	UNSUPPORTEDOPERATION_UNABLECROSSFINANCE = "UnsupportedOperation.UnableCrossFinance"

	// 未分配IPv6网段。
	UNSUPPORTEDOPERATION_UNASSIGNCIDRBLOCK = "UnsupportedOperation.UnassignCidrBlock"

	// 未绑定EIP。
	UNSUPPORTEDOPERATION_UNBINDEIP = "UnsupportedOperation.UnbindEIP"

	// 账户还有未支付订单，请先完成付款。
	UNSUPPORTEDOPERATION_UNPAIDORDERALREADYEXISTS = "UnsupportedOperation.UnpaidOrderAlreadyExists"

	// 不支持绑定LocalZone弹性公网IP。
	UNSUPPORTEDOPERATION_UNSUPPORTEDBINDLOCALZONEEIP = "UnsupportedOperation.UnsupportedBindLocalZoneEIP"

	// 指定机型不支持弹性网卡。
	UNSUPPORTEDOPERATION_UNSUPPORTEDINSTANCEFAMILY = "UnsupportedOperation.UnsupportedInstanceFamily"

	// 暂无法在此国家/地区提供该服务。
	UNSUPPORTEDOPERATION_UNSUPPORTEDREGION = "UnsupportedOperation.UnsupportedRegion"

	// 当前用户付费类型不支持创建所选付费类型的云联网。
	UNSUPPORTEDOPERATION_USERANDCCNCHARGETYPENOTMATCH = "UnsupportedOperation.UserAndCcnChargeTypeNotMatch"

	// 指定安全组规则版本号和当前最新版本不一致。
	UNSUPPORTEDOPERATION_VERSIONMISMATCH = "UnsupportedOperation.VersionMismatch"

	// 资源不属于同一个VPC。
	UNSUPPORTEDOPERATION_VPCMISMATCH = "UnsupportedOperation.VpcMismatch"

	// 指定资源在不同的可用区。
	UNSUPPORTEDOPERATION_ZONEMISMATCH = "UnsupportedOperation.ZoneMismatch"

	// 已经达到指定区域vpc资源申请数量上限。
	VPCLIMITEXCEEDED = "VpcLimitExceeded"
)
