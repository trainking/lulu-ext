package matchx

import "time"

// run 执行匹配
func (mq *MatchQueue) run() {
	tiemOut := time.NewTicker(mq.realTimeout)
	for {
		select {
		case <-mq.closeChan:
			return
		case p := <-mq.matchDel:
			for e := mq.matching.Front(); e != nil; e = e.Next() {
				eV := e.Value.(uint64)
				if eV == p {
					mq.matching.Remove(e)
					delete(mq.matchValue, p)
					break
				}
			}
		case p := <-mq.matchAdd:
			mq.matching.PushBack(p)
			mq.matchValue[p.UserID] = p
			if mq.matching.Len() >= mq.callback.GetSuccessNum() {
				mq.groupReal()
			}
		case <-tiemOut.C:
			if mq.matching.Len() > 0 {
				length := mq.matching.Len()
				for i := 0; i < length; i++ {
					item := mq.matching.Front()
					userID := item.Value.(uint64)
					if mp, ok := mq.matchValue[userID]; ok {
						mq.matching.Remove(item)
						delete(mq.matchValue, userID)
						mq.GroupAI(mp)
					}
				}
			}
		}
	}
}

// groupReal 组真人匹配
func (mq *MatchQueue) groupReal() {
	successNum := mq.callback.GetSuccessNum()
	if mq.matching.Len() >= successNum {
		var players []*MatchPlayer
		for item := mq.matching.Front(); item != nil; item = item.Next() {
			iv := item.Value.(uint64)
			if mp, ok := mq.matchValue[iv]; ok {
				players = append(players, mp)
			}
			mq.matching.Remove(item)
			delete(mq.matchValue, iv)
			if len(players) == successNum {
				mq.success <- &MatchGroup{Players: players}
				return
			}
		}

		// 未组成，则将所有玩家放回
		for _, p := range players {
			mq.matching.PushBack(p)
			mq.matchValue[p.UserID] = p
		}
	}
}

// groupAI 组合机器人匹配
func (mq *MatchQueue) GroupAI(p *MatchPlayer) {
	successNum := mq.callback.GetSuccessNum()
	robots, err := mq.callback.CallRobots(successNum - 1)
	if err != nil {
		return
	}

	if len(robots) == successNum-1 {
		players := make([]*MatchPlayer, 0, successNum)
		players = append(players, p)
		players = append(players, robots...)
		mq.success <- &MatchGroup{Players: players}
	}
}

// Add 增加匹配玩家
func (mq *MatchQueue) Add(p *MatchPlayer) {
	mq.matchAdd <- p
}

// Del 删除匹配玩家
func (mq *MatchQueue) Del(p uint64) {
	mq.matchDel <- p
}

// Close 关闭匹配队列
func (mq *MatchQueue) Close() {
	close(mq.closeChan)
}
