// Package vote 提供对投票的创建、进行、获取操作
package vote

import "github.com/qiulaidongfeng/nonamevote/internal/utils"

var notexist = utils.GenTipText("不存在的投票", "/", "返回首页")
var needlogin = utils.GenTipText("需要登录才能投票", "/login", "前往登录页")
