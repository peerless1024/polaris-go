/**
 * Tencent is pleased to support the open source community by making polaris-go available.
 *
 * Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
 *
 * Licensed under the BSD 3-Clause License (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * https://opensource.org/licenses/BSD-3-Clause
 *
 * Unless required by applicable law or agreed to in writing, software distributed
 * under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
 * CONDITIONS OF ANY KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 */

package ratelimit

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"gopkg.in/check.v1"

	"github.com/polarismesh/polaris-go/api"
	"github.com/polarismesh/polaris-go/pkg/config"
	"github.com/polarismesh/polaris-go/pkg/model"
)

// 远程正常用例测试
type RemoteNormalTestingSuite struct {
	CommonRateLimitSuite
}

// 用例名称
func (rt *RemoteNormalTestingSuite) GetName() string {
	return "RemoteNormalTestingSuite"
}

// SetUpSuite 启动测试套程序
func (rt *RemoteNormalTestingSuite) SetUpSuite(c *check.C) {
	rt.CommonRateLimitSuite.SetUpSuite(c, true)
}

// SetUpSuite 结束测试套程序
func (rt *RemoteNormalTestingSuite) TearDownSuite(c *check.C) {
	rt.CommonRateLimitSuite.TearDownSuite(c, rt)
}

// 带标识的结果
type IndexResult struct {
	index int
	label string
	code  model.QuotaResultCode
}

// 测试远程精准匹配限流
func (rt *RemoteNormalTestingSuite) TestRemoteTwoDuration(c *check.C) {
	log.Printf("Start TestRemoteTwoDuration")
	// 多个线程，然后每个线程一个client，每个client跑相同的labels
	workerCount := 4
	wg := &sync.WaitGroup{}
	wg.Add(workerCount)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	codeChan := make(chan IndexResult)
	var calledCount int64
	for i := 0; i < workerCount; i++ {
		go func(idx int) {
			defer wg.Done()
			cfg := config.NewDefaultConfiguration([]string{mockDiscoverAddress})
			limitAPI, err := api.NewLimitAPIByConfig(cfg)
			c.Assert(err, check.IsNil)
			defer limitAPI.Destroy()
			once := &sync.Once{}
			for {
				select {
				case <-ctx.Done():
					return
				default:
					resp := doSingleGetQuota(c, limitAPI, RemoteTestSvcName, "query",
						map[string]string{labelUin: "007"})
					atomic.AddInt64(&calledCount, 1)
					codeChan <- IndexResult{
						index: idx,
						code:  resp.Code,
					}
					once.Do(func() {
						time.Sleep(100 * time.Millisecond)
					})
					time.Sleep(5 * time.Millisecond)
				}
			}
		}(i)
	}
	allocatedPerSeconds := make([]int, 0, 20)
	var allocatedTotal int
	ctx1, cancel1 := context.WithCancel(context.Background())
	go func() {
		var totalPerSecond int
		var rejectIdx = make(map[int]bool)
		for {
			select {
			case <-ctx1.Done():
				return
			case idxResult := <-codeChan:
				if idxResult.code == api.QuotaResultOk {
					allocatedTotal++
					totalPerSecond++
					rejectIdx = make(map[int]bool)
				} else {
					rejectIdx[idxResult.index] = true
					if len(rejectIdx) >= workerCount && totalPerSecond > 0 {
						allocatedPerSeconds = append(allocatedPerSeconds, totalPerSecond)
						totalPerSecond = 0
					}
				}
			}
		}
	}()
	wg.Wait()
	cancel1()
	fmt.Printf("calledCount is %d\n", calledCount)
	fmt.Printf("allocatedPerSeconds is %v\n", allocatedPerSeconds)
	fmt.Printf("allocatedTotal is %d\n", allocatedTotal)
	for i, allocatedPerSecond := range allocatedPerSeconds {
		if i == 0 {
			// 头部因为时间窗对齐原因，有可能出现不为100
			continue
		}
		if allocatedPerSecond < 100 {
			// 中间出现了10s区间限流的情况，屏蔽
			continue
		}
		c.Assert(allocatedPerSecond >= 150 && allocatedPerSecond <= 230, check.Equals, true)
	}
	c.Assert(allocatedTotal >= 800 && allocatedTotal <= 1650, check.Equals, true)
}

// 测试正则表达式的uin限流
func (rt *RemoteNormalTestingSuite) TestRemoteRegexV2(c *check.C) {
	log.Printf("Start TestRemoteRegexV2")
	// 多个线程，然后每个线程一个client，每个client跑相同的labels
	workerCount := 4
	wg := &sync.WaitGroup{}
	wg.Add(workerCount)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	appIds := []string{"appId1", "appId2"}

	codeChan := make(chan IndexResult)
	var calledCount int64
	for i := 0; i < workerCount; i++ {
		go func(idx int) {
			defer wg.Done()
			cfg := config.NewDefaultConfiguration([]string{mockDiscoverAddress})
			limitAPI, err := api.NewLimitAPIByConfig(cfg)
			c.Assert(err, check.IsNil)
			defer limitAPI.Destroy()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					for _, appId := range appIds {
						resp := doSingleGetQuota(c, limitAPI, RemoteTestSvcName, "",
							map[string]string{labelAppId: appId})
						atomic.AddInt64(&calledCount, 1)
						codeChan <- IndexResult{
							index: idx,
							label: appId,
							code:  resp.Code,
						}
						time.Sleep(5 * time.Millisecond)
					}
				}
			}
		}(i)
	}
	appIdAllocatedPerSeconds := make(map[string][]int)
	for _, appId := range appIds {
		appIdAllocatedPerSeconds[appId] = make([]int, 0)
	}
	ctx1, cancel1 := context.WithCancel(context.Background())
	go func() {
		var appIdTotalPerSecond = make(map[string]int)
		var appIdRejectIdx = make(map[string]map[int]bool)
		for _, appId := range appIds {
			appIdRejectIdx[appId] = make(map[int]bool)
			appIdTotalPerSecond[appId] = 0
		}
		for {
			select {
			case <-ctx1.Done():
				return
			case idxResult := <-codeChan:
				rejectIdx := appIdRejectIdx[idxResult.label]
				totalPerSecond := appIdTotalPerSecond[idxResult.label]
				allocatedPerSeconds := appIdAllocatedPerSeconds[idxResult.label]
				if idxResult.code == api.QuotaResultOk {
					appIdTotalPerSecond[idxResult.label] = totalPerSecond + 1
					if len(rejectIdx) > 0 {
						appIdRejectIdx[idxResult.label] = make(map[int]bool)
					}
				} else {
					rejectIdx[idxResult.index] = true
					if len(rejectIdx) >= workerCount && totalPerSecond > 0 {
						allocatedPerSeconds = append(allocatedPerSeconds, totalPerSecond)
						appIdAllocatedPerSeconds[idxResult.label] = allocatedPerSeconds
						appIdTotalPerSecond[idxResult.label] = 0
					}
				}
			}
		}
	}()
	wg.Wait()
	cancel1()
	fmt.Printf("calledCount is %d\n", calledCount)
	fmt.Printf("appIdAllocatedPerSeconds is %v\n", appIdAllocatedPerSeconds)
	for _, allocatedPerSeconds := range appIdAllocatedPerSeconds {
		for i, allocatedPerSecond := range allocatedPerSeconds {
			if i == 0 {
				// 头部因为时间窗对齐原因，有可能出现不为100
				continue
			}
			c.Assert(allocatedPerSecond >= 90 && allocatedPerSecond <= 120, check.Equals, true)
		}
	}
}

// 测试正则表达式的uin限流
func (rt *RemoteNormalTestingSuite) TestRemoteRegexCombineV2(c *check.C) {
	log.Printf("Start TestRemoteRegexCombineV2")
	workerCount := 4
	wg := &sync.WaitGroup{}
	wg.Add(workerCount)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	appIds := []string{"appId1", "appId2"}

	codeChan := make(chan IndexResult)
	var calledCount int64
	for i := 0; i < workerCount; i++ {
		go func(idx int) {
			defer wg.Done()
			cfg := config.NewDefaultConfiguration([]string{mockDiscoverAddress})
			limitAPI, err := api.NewLimitAPIByConfig(cfg)
			c.Assert(err, check.IsNil)
			defer limitAPI.Destroy()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					for _, appId := range appIds {
						resp := doSingleGetQuota(c, limitAPI, RemoteTestSvcName, "",
							map[string]string{labelTestUin: appId})
						atomic.AddInt64(&calledCount, 1)
						codeChan <- IndexResult{
							index: idx,
							label: appId,
							code:  resp.Code,
						}
						time.Sleep(5 * time.Millisecond)
					}
				}
			}
		}(i)
	}
	allocatedPerSeconds := make([]int, 0, 20)
	ctx1, cancel1 := context.WithCancel(context.Background())
	go func() {
		var totalPerSecond int
		var rejectIdx = make(map[int]bool)
		for {
			select {
			case <-ctx1.Done():
				return
			case idxResult := <-codeChan:
				if idxResult.code == api.QuotaResultOk {
					totalPerSecond++
					rejectIdx = make(map[int]bool)
				} else {
					rejectIdx[idxResult.index] = true
					if len(rejectIdx) >= workerCount && totalPerSecond > 0 {
						allocatedPerSeconds = append(allocatedPerSeconds, totalPerSecond)
						totalPerSecond = 0
					}
				}
			}
		}
	}()
	wg.Wait()
	cancel1()
	fmt.Printf("calledCount is %d\n", calledCount)
	fmt.Printf("allocatedPerSeconds is %v\n", allocatedPerSeconds)

	for i, allocatedPerSecond := range allocatedPerSeconds {
		if i == 0 {
			// 头部因为时间窗对齐原因，有可能出现不为100
			continue
		}
		c.Assert(allocatedPerSecond >= 280 && allocatedPerSecond <= 350, check.Equals, true)
	}
}

// 测试单机均摊模式限流
func (rt *RemoteNormalTestingSuite) TestRemoteShareEqually(c *check.C) {
	log.Printf("Start TestRemoteShareEqually")
	workerCount := 10
	wg := &sync.WaitGroup{}
	wg.Add(workerCount)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	codeChan := make(chan IndexResult)
	var calledCount int64
	startTime := model.CurrentMillisecond()
	for i := 0; i < workerCount; i++ {
		go func(idx int) {
			defer wg.Done()
			cfg := config.NewDefaultConfiguration([]string{mockDiscoverAddress})
			limitAPI, err := api.NewLimitAPIByConfig(cfg)
			c.Assert(err, check.IsNil)
			defer limitAPI.Destroy()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					resp := doSingleGetQuota(c, limitAPI, RemoteTestSvcName, "",
						map[string]string{"appIdShare": "appShare"})
					atomic.AddInt64(&calledCount, 1)
					curTime := model.CurrentMillisecond()
					if curTime-startTime >= 1000 {
						// 前500ms是上下线，不计算
						codeChan <- IndexResult{
							index: idx,
							code:  resp.Code,
						}
					}
					time.Sleep(10 * time.Millisecond)
				}
			}
		}(i)
	}
	allocatedPerSeconds := make([]int, 0, 20)
	ctx1, cancel1 := context.WithCancel(context.Background())
	go func() {
		var totalPerSecond int
		var rejectIdx = make(map[int]bool)
		for {
			select {
			case <-ctx1.Done():
				return
			case idxResult := <-codeChan:
				if idxResult.code == api.QuotaResultOk {
					totalPerSecond++
					rejectIdx = make(map[int]bool)
				} else {
					rejectIdx[idxResult.index] = true
					if len(rejectIdx) >= workerCount && totalPerSecond > 0 {
						allocatedPerSeconds = append(allocatedPerSeconds, totalPerSecond)
						totalPerSecond = 0
					}
				}
			}
		}
	}()
	wg.Wait()
	cancel1()
	fmt.Printf("calledCount is %d\n", calledCount)
	fmt.Printf("allocatedPerSeconds is %v\n", allocatedPerSeconds)

	for i, allocatedPerSecond := range allocatedPerSeconds {
		if i == 0 {
			// 头部因为时间窗对齐原因，有可能出现不为100
			continue
		}
		c.Assert(allocatedPerSecond >= 170 && allocatedPerSecond <= 260, check.Equals, true)
	}
}

// TestReproducePanic 专门用于复现 async_flow.go 中的空指针 panic
// 当 GetQuota 返回错误时，future 为 nil，但代码仍然调用 future.GetImmediately() 导致 panic
//
// 复现方法（不修改 assist.go）：
// 1. 配置一个不存在的 discover server 地址
// 2. 设置极短的超时时间（1ms）
// 3. 调用 GetQuota，SyncGetResources 会因超时返回 ErrCodeAPITimeoutError
// 4. lookupRateLimitWindow 返回 nil, err
// 5. GetQuota 返回 nil, err
// 6. AsyncGetQuota 调用 future.GetImmediately() 触发 panic
//
// 修改：确保缓存获取失败，走到 sync_flow.go:193 而不是 180
func (rt *RemoteNormalTestingSuite) TestReproducePanic(c *check.C) {
	fmt.Println("\n========== Start TestReproducePanic ==========")
	fmt.Println("[Info] 此测试用例用于复现 async_flow.go:39 中的空指针 panic")
	fmt.Println("[Info] 使用不存在的服务器地址 + 极短超时时间触发 SyncGetResources 超时错误")
	fmt.Println("[Info] 彻底禁用缓存，确保走到 sync_flow.go:193 返回 ErrCodeAPITimeoutError")
	fmt.Println("[Info] 这会导致 GetQuota 返回 nil, err，然后 future.GetImmediately() 触发 panic")

	workerCount := 4
	wg := &sync.WaitGroup{}
	wg.Add(workerCount)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var calledCount int64
	var panicCount int64

	fmt.Printf("[Worker] Starting %d worker goroutines...\n", workerCount)
	for i := 0; i < workerCount; i++ {
		go func(idx int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panicCount, 1)
					fmt.Printf("[Worker-%d] PANIC captured: %v\n", idx, r)
					// 打印详细的堆栈跟踪信息
					fmt.Printf("[Worker-%d] Stack trace:\n", idx)
					debug.PrintStack()
					fmt.Printf("[Worker-%d] End of stack trace\n", idx)
				}
			}()

			fmt.Printf("[Worker-%d] Initializing limitAPI with non-existent server...\n", idx)
			// 使用不存在的服务器地址（TEST-NET-1 范围，不可路由）
			cfg := config.NewDefaultConfiguration([]string{"192.0.2.1:8091"})
			// 设置极短的超时时间，使 SyncGetResources 快速超时
			cfg.GetGlobal().GetAPI().SetTimeout(1 * time.Millisecond)
			cfg.GetGlobal().GetAPI().SetMaxRetryTimes(1) // 设置重试次数，确保能走完重试流程
			// 彻底禁用缓存
			cfg.GetConsumer().GetLocalCache().SetPersistEnable(false)
			cfg.GetConsumer().GetLocalCache().SetStartUseFileCache(false) // 不使用文件缓存
			// 使用最小允许的缓存过期时间
			cfg.GetConsumer().GetLocalCache().SetServiceExpireTime(5 * time.Second) // 最小允许的过期时间

			// 删除所有相关的缓存文件，确保缓存获取失败
			rt.removeAllServiceCacheFiles(namespaceTest, "AnyServicea")
			rt.removeAllServiceCacheFiles(namespaceTest, "TimeoutTestService")
			rt.removeAllServiceCacheFiles(namespaceTest, "NonExistentService")
			// 删除 polaris 备份目录下的所有缓存文件
			rt.removeAllServiceCacheFiles("Polaris", "polaris.metric.test.ide")
			rt.removeAllServiceCacheFiles("Polaris", "polaris-discover")

			limitAPI, err := api.NewLimitAPIByConfig(cfg)
			if err != nil {
				fmt.Printf("[Worker-%d] ERROR: Failed to create limitAPI: %v\n", idx, err)
				return
			}
			defer limitAPI.Destroy()
			fmt.Printf("[Worker-%d] LimitAPI initialized successfully\n", idx)

			for {
				select {
				case <-ctx.Done():
					fmt.Printf("[Worker-%d] Context done, total calls: %d\n", idx, atomic.LoadInt64(&calledCount))
					return
				default:
					// 使用不同的服务名，避免缓存命中
					serviceName := fmt.Sprintf("AnyServicea_%d_%d", idx, atomic.LoadInt64(&calledCount)%10)

					// 对任意服务调用 GetQuota
					// SyncGetResources 会因连接不存在的服务器超时而重试，最终缓存获取失败
					// 走到 sync_flow.go:193 返回 ErrCodeAPITimeoutError
					// lookupRateLimitWindow 会传递这个错误（因为它不是 ErrCodeServiceNotFound）
					// GetQuota 返回 nil, err
					// AsyncGetQuota 调用 future.GetImmediately() 触发 panic
					quotaReq := api.NewQuotaRequest()
					quotaReq.SetNamespace(namespaceTest)
					quotaReq.SetService(serviceName) // 使用动态服务名，确保缓存清空
					quotaReq.SetMethod("anyMethod")

					_, err := limitAPI.GetQuota(quotaReq)
					currentTotal := atomic.AddInt64(&calledCount, 1)

					if currentTotal%10 == 0 {
						if err != nil {
							fmt.Printf("[Worker-%d] Progress: total=%d, error=%v\n", idx, currentTotal, err)
						} else {
							fmt.Printf("[Worker-%d] Progress: total=%d, success\n", idx, currentTotal)
						}
					}

					time.Sleep(100 * time.Millisecond)
				}
			}
		}(i)
	}

	fmt.Printf("[Main] Waiting for all workers to complete...\n")
	wg.Wait()

	fmt.Printf("\n========== Test Results ==========\n")
	fmt.Printf("[Result] Total called count: %d\n", calledCount)
	fmt.Printf("[Result] Total panic count: %d\n", panicCount)

	if panicCount > 0 {
		fmt.Printf("[Result] ✅ Successfully reproduced the panic!\n")
		fmt.Printf("[Result] The panic occurs at async_flow.go:39 when future.GetImmediately() is called on nil\n")
	} else {
		fmt.Printf("[Result] ❌ Panic was not triggered (bug may have been fixed)\n")
	}
	fmt.Printf("========== TestReproducePanic Completed ==========\n\n")
}

// removeAllServiceCacheFiles 删除指定服务和命名空间的所有缓存文件
func (rt *RemoteNormalTestingSuite) removeAllServiceCacheFiles(namespace, service string) {
	backupDir := "./polaris/backup"

	// 需要删除的缓存文件类型
	cacheTypes := []string{"instance", "routing", "rate_limiting"}

	for _, cacheType := range cacheTypes {
		cacheFileName := filepath.Join(backupDir, fmt.Sprintf("svc#%s#%s#%s.json", namespace, service, cacheType))

		// 检查文件是否存在并删除
		if _, err := os.Stat(cacheFileName); err == nil {
			if removeErr := os.Remove(cacheFileName); removeErr != nil {
				log.Printf("Warning: failed to remove cache file %s: %v", cacheFileName, removeErr)
			} else {
				log.Printf("Info: successfully removed cache file %s", cacheFileName)
			}
		}
	}
}
