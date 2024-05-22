/*
 * @LastEditTime: 2021-05-06 14:06:09
 * @LastEditors: trainking
 * @Description:
 * @FilePath: \golang\matchx\matchx.go
 * match group tools.
 */
package matchx

import (
	"container/list"
	"time"
)

type (
	// MatchPlayer 匹配的玩家
	MatchPlayer struct {
		UserID   uint64      // 此玩家的ID，唯一标识
		UserData interface{} // 玩家数据
		IsRobot  bool        // 是否是机器人
	}

	// MatchGroup 匹配的玩家组
	MatchGroup struct {
		Players []*MatchPlayer
	}

	// MatchCallback 匹配回调
	MatchCallback interface {
		// CallRobots 召唤指定数量的机器人
		CallRobots(num int) ([]*MatchPlayer, error)

		// GetSuccessNum 获取成功匹配的玩家数量
		GetSuccessNum() int
	}

	// MatchQueue 匹配队列
	MatchQueue struct {
		closeChan   chan struct{}           // 关闭信号
		matchValue  map[uint64]*MatchPlayer // 匹配玩家映射数据
		matching    *list.List              // 匹配队列
		matchAdd    chan *MatchPlayer       // 进入匹配队列
		matchDel    chan uint64             // 离开匹配队列
		success     chan *MatchGroup        // 成功匹配的队列
		callback    MatchCallback           // 成功匹配的回调
		realTimeout time.Duration           // 真人匹配超时时间
	}
)

// NewMatchQueue 创建匹配队列
func NewMatchQueue(callback MatchCallback, successWait int, realTimeout time.Duration) *MatchQueue {
	mq := &MatchQueue{
		closeChan:   make(chan struct{}),
		matchValue:  make(map[uint64]*MatchPlayer),
		matching:    list.New(),
		matchAdd:    make(chan *MatchPlayer),
		matchDel:    make(chan uint64),
		success:     make(chan *MatchGroup, successWait),
		callback:    callback,
		realTimeout: realTimeout,
	}

	go mq.run()

	return mq
}
