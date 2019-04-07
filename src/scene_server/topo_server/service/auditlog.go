/*
 * Tencent is pleased to support the open source community by making 蓝鲸 available.
 * Copyright (C) 2017-2018 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 */

package service

import (
	"configcenter/src/common"
	"configcenter/src/common/blog"
	"configcenter/src/common/mapstr"
	"configcenter/src/common/metadata"
	"configcenter/src/common/util"
	"configcenter/src/scene_server/topo_server/core/types"
	"fmt"
)

const CCTimeTypeParseFlag = "cc_time_type"

// AuditQuery search audit logs
func (s *Service) AuditQuery(params types.ContextParams, pathParams, queryParams ParamsGetter, data mapstr.MapStr) (interface{}, error) {
	query := &metadata.QueryInput{}
	if err := data.MarshalJSONInto(query); nil != err {
		blog.Errorf("[audit] failed to parse the input (%#v), error info is %s", data, err.Error())
		return nil, params.Err.New(common.CCErrCommJSONUnmarshalFailed, err.Error())
	}

	queryCondition := query.Condition
	if nil == queryCondition {
		query.Condition = common.KvMap{common.BKOwnerIDField: params.SupplierAccount}
	} else {
		cond := queryCondition.(map[string]interface{})
		times, ok := cond[common.BKOpTimeField].([]interface{})
		if ok {
			if 2 != len(times) {
				blog.Errorf("search operation log input params times error, info: %v", times)
				return nil, params.Err.Error(common.CCErrCommParamsInvalid)
			}

			cond[common.BKOpTimeField] = common.KvMap{
				"$gte":              times[0],
				"$lte":              times[1],
				CCTimeTypeParseFlag: "1",
			}
		}
		cond[common.BKOwnerIDField] = params.SupplierAccount
		query.Condition = cond
	}
	if 0 == query.Limit {
		query.Limit = common.BKDefaultLimit
	}

	// add auth filter condition
	var businessID int64
	bizID, exist := query.Condition.(map[string]interface{})[common.BKAppIDField]
	if exist == true {
		id, err := util.GetInt64ByInterface(bizID)
		if err != nil {
			blog.Errorf("%s field in query condition but parse int failed, err: %+v", common.BKAppIDField, err)
		}
		businessID = id
	}

	authCondition, hasAuthorization, err := s.AuthManager.MakeAuthorizedAuditListCondition(params.Context, params.Header, businessID)
	if err != nil {
		blog.Errorf("make audit query condition from auth failed, %+v", err)
		return nil, fmt.Errorf("make audit query condition from auth failed, %+v", err)
	}
	if hasAuthorization == false {
		blog.Errorf("user %+v has no authorization on audit", params.User)
		return nil, nil
	}
	blog.V(5).Infof("auth condition is: %+v", authCondition)
	
	
	mergedCondition := query.Condition.(mapstr.MapStr)
	mergedCondition.Merge(authCondition.ToMapStr())
	
	query.Condition = mergedCondition.ToMapInterface()
	
	blog.InfoJSON("AuditOperation parameter: %s", query)
	return s.Core.AuditOperation().Query(params, query)
}
