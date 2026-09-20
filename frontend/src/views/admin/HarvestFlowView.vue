<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 class="text-xl font-semibold text-gray-900 dark:text-white">{{ t('admin.harvestFlow.title') }}</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.harvestFlow.description') }}</p>
        </div>
        <div class="flex items-center gap-3">
          <button type="button" class="btn btn-secondary btn-sm" @click="toggleAutoConfigPanel">
            <Icon name="cog" size="sm" class="mr-1" />
            {{ autoConfigOpen ? '收起自动打票配置' : '⚙️ 自定义自动打票配置' }}
          </button>
          <label class="inline-flex items-center gap-2 text-sm text-gray-600 dark:text-gray-300">
            <input v-model="autoRefresh" type="checkbox" class="rounded border-gray-300 text-primary-600" />
            {{ t('admin.harvestFlow.autoRefresh') }}
          </label>
          <button type="button" class="btn btn-secondary btn-sm" :disabled="loading" @click="fetchFlow">
            <Icon name="refresh" size="sm" class="mr-1" :class="{ 'animate-spin': loading || refreshing }" />
            {{ t('common.refresh') }}
          </button>
        </div>
      </div>

      <div v-if="errorMessage" class="rounded-2xl bg-red-50 p-4 text-sm text-red-600 dark:bg-red-900/20 dark:text-red-400">
        {{ errorMessage }}
      </div>

      <div v-if="loading && !snapshot" class="flex items-center justify-center py-16">
        <LoadingSpinner />
      </div>

      <template v-else-if="snapshot">
        <div class="card overflow-hidden p-5">
          <div class="mb-5 flex flex-wrap items-center justify-between gap-3">
            <div class="flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
              <span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5" :class="snapshot.harvest.enabled ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300' : 'bg-gray-100 text-gray-500 dark:bg-dark-700'">
                {{ snapshot.harvest.enabled ? t('admin.harvestFlow.harvestOn') : t('admin.harvestFlow.harvestOff') }}
              </span>
              <span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5" :class="snapshot.harvest.fail_closed ? 'bg-rose-50 text-rose-700 dark:bg-rose-900/30 dark:text-rose-300' : 'bg-amber-50 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'">
                {{ snapshot.harvest.fail_closed ? t('admin.harvestFlow.failClosed') : t('admin.harvestFlow.failOpen') }}
              </span>
              <span>{{ t('admin.harvestFlow.strategy') }} {{ snapshot.harvest.strategy }}</span>
              <span>{{ t('admin.harvestFlow.scope') }} {{ scopeLabel(snapshot.harvest.scope_mode, snapshot.harvest.account_policy, snapshot.harvest.group_ids) }}</span>
              <span>{{ t('admin.harvestFlow.interval') }} {{ t('admin.harvestFlow.seconds', { n: snapshot.harvest.probe_interval_seconds }) }}</span>
              <span>{{ t('admin.harvestFlow.cooldown') }} {{ t('admin.harvestFlow.seconds', { n: snapshot.harvest.cooldown_seconds }) }}</span>
              <span>{{ t('admin.harvestFlow.targetLength', { n: snapshot.harvest.target_length }) }}</span>
              <span v-if="snapshot.harvest.models?.length">{{ t('admin.harvestFlow.models') }} {{ snapshot.harvest.models.join(' / ') }}</span>
            </div>
            <p v-if="lastUpdated" class="text-xs text-gray-400">{{ t('admin.harvestFlow.lastUpdated', { time: lastUpdated }) }}</p>
          </div>

          <ol class="grid grid-cols-1 gap-3 md:grid-cols-5">
            <li v-for="(stage, index) in snapshot.stages" :key="stage.id" class="relative">
              <div class="h-full rounded-2xl border p-4 transition-colors" :class="stageCardClass(stage.status)">
                <div class="mb-3 flex items-center justify-between">
                  <div class="flex h-9 w-9 items-center justify-center rounded-xl" :class="stageIconClass(stage.status)">
                    <Icon :name="stageIcon(stage.id)" size="sm" />
                  </div>
                  <span class="text-[11px] font-medium uppercase tracking-wide" :class="stageTextClass(stage.status)">
                    {{ statusLabel(stage.status) }}
                  </span>
                </div>
                <p class="text-sm font-semibold text-gray-900 dark:text-white">{{ t(`admin.harvestFlow.stages.${stage.id}`) }}</p>
                <p class="mt-1 break-all font-mono text-xs text-gray-500 dark:text-gray-400">{{ stageDetail(stage) }}</p>
                <p v-if="stage.model || stage.at || stage.http_status" class="mt-2 text-[11px] text-gray-400">
                  <span v-if="stage.model">{{ stage.model }}</span>
                  <span v-if="stage.http_status"> · HTTP {{ stage.http_status }}</span>
                  <span v-if="(stage.model || stage.http_status) && stage.at"> · </span>
                  <span v-if="stage.at">{{ formatClock(stage.at) }}</span>
                </p>
              </div>
              <div v-if="index < snapshot.stages.length - 1" class="pointer-events-none absolute right-[-10px] top-1/2 hidden h-px w-5 bg-gradient-to-r from-gray-300 to-transparent md:block dark:from-dark-600" />
            </li>
          </ol>
        </div>

        <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
          <div class="card p-4">
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.harvestFlow.sidecar') }}</p>
            <p class="mt-1 text-sm font-semibold" :class="snapshot.sidecar.reachable ? 'text-emerald-600 dark:text-emerald-400' : 'text-rose-600 dark:text-rose-400'">
              {{ snapshot.sidecar.reachable ? t('admin.harvestFlow.sidecarReachable') : t('admin.harvestFlow.sidecarOffline') }}
            </p>
            <p class="mt-2 break-all font-mono text-xs text-gray-500 dark:text-gray-400">
              {{ snapshot.sidecar.now || (snapshot.sidecar.reachable ? t('admin.harvestFlow.poolOnline', { n: snapshot.sidecar.all_count || 0 }) : t('admin.harvestFlow.waitingSidecar')) }}
            </p>
            <p class="mt-1 text-xs text-gray-400">{{ t('admin.harvestFlow.nodePool') }} {{ snapshot.sidecar.all_count || 0 }} · {{ snapshot.sidecar.group || 'CODEX-ROTATE' }}</p>
            <p v-if="snapshot.sidecar.error" class="mt-1 text-xs text-rose-500">{{ snapshot.sidecar.error }}</p>
          </div>
          <div class="card p-4">
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.harvestFlow.ready') }}</p>
            <p class="mt-1 text-2xl font-bold text-gray-900 dark:text-white">{{ snapshot.counts.tickets_ready }}</p>
            <p class="text-xs text-rose-500">{{ t('admin.harvestFlow.paused') }} {{ snapshot.counts.tickets_blocked }}</p>
          </div>
          <div class="card p-4">
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.harvestFlow.probeHit') }} / {{ t('admin.harvestFlow.probeMiss') }}</p>
            <p class="mt-1 text-2xl font-bold text-gray-900 dark:text-white">{{ snapshot.counts.probe_hit }} / {{ snapshot.counts.probe_miss }}</p>
            <p class="text-xs text-gray-400">{{ t('admin.harvestFlow.ticketAccept') }} {{ snapshot.counts.ticket_accept }} · {{ t('admin.harvestFlow.ticketReject') }} {{ snapshot.counts.ticket_reject }}</p>
          </div>
          <div class="card p-4">
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.harvestFlow.selectOk') }}</p>
            <p class="mt-1 text-2xl font-bold text-gray-900 dark:text-white">{{ snapshot.counts.select_ok }}</p>
            <p class="text-xs text-gray-400">{{ t('admin.harvestFlow.selectSkip') }} {{ snapshot.counts.select_skip }} · {{ t('admin.harvestFlow.selectFail') }} {{ snapshot.counts.select_fail }}</p>
            <p v-if="snapshot.harvest.harvest_proxy" class="mt-2 truncate font-mono text-[11px] text-gray-400">{{ snapshot.harvest.harvest_proxy }}</p>
          </div>
        </div>

        <!-- ⚙️ 自动打票参数配置面板 (可展开/折叠) -->
        <div v-show="autoConfigOpen" class="card overflow-hidden border border-emerald-200 dark:border-emerald-800/40 p-5 bg-emerald-50/20 dark:bg-emerald-950/10">
          <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
            <div>
              <h2 class="text-sm font-semibold text-gray-900 dark:text-white flex items-center gap-2">
                <span>⚙️ 自动打票参数配置</span>
                <span class="text-[10px] bg-emerald-100 text-emerald-800 dark:bg-emerald-900/50 dark:text-emerald-300 px-2 py-0.5 rounded font-medium">实时保存生效</span>
              </h2>
              <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">直接调整后台自动巡检节拍与并发，保存后下轮打票周期自动对齐。</p>
            </div>
            <div class="flex items-center gap-2">
              <button type="button" class="btn btn-secondary btn-sm" @click="resetAutoConfig">恢复默认值</button>
              <button type="button" class="btn btn-primary btn-sm bg-emerald-600 hover:bg-emerald-700 text-white" :disabled="savingAutoConfig" @click="saveAutoConfig">
                {{ savingAutoConfig ? '保存中...' : '💾 保存参数' }}
              </button>
            </div>
          </div>

          <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-5">
            <div>
              <label class="block text-xs font-medium text-gray-700 dark:text-gray-300">检测账号票据的周期</label>
              <div class="relative mt-1">
                <input v-model.number="autoConfigForm.probe_interval_seconds" type="number" min="10" max="1800" class="input input-sm w-full font-mono pr-7 text-xs" />
                <span class="absolute right-2 top-2 text-[11px] text-gray-400">秒</span>
              </div>
              <p class="mt-1 text-[11px] text-gray-400">全池巡检等待 (10~1800s)</p>
            </div>
            <div>
              <label class="block text-xs font-medium text-gray-700 dark:text-gray-300">每轮打号并发</label>
              <div class="relative mt-1">
                <input v-model.number="autoConfigForm.max_probes_per_round" type="number" min="1" max="50" class="input input-sm w-full font-mono pr-8 text-xs" />
                <span class="absolute right-2 top-2 text-[11px] text-gray-400">个号</span>
              </div>
              <p class="mt-1 text-[11px] text-gray-400">每轮最大账号数 (1~50)</p>
            </div>
            <div>
              <label class="block text-xs font-medium text-gray-700 dark:text-gray-300">失败冷却</label>
              <div class="relative mt-1">
                <input v-model.number="autoConfigForm.cooldown_seconds" type="number" min="5" max="600" class="input input-sm w-full font-mono pr-7 text-xs" />
                <span class="absolute right-2 top-2 text-[11px] text-gray-400">秒</span>
              </div>
              <p class="mt-1 text-[11px] text-gray-400">未中锁定冷却 (5~600s)</p>
            </div>
            <div>
              <label class="block text-xs font-medium text-gray-700 dark:text-gray-300">单次探针超时</label>
              <div class="relative mt-1">
                <input v-model.number="autoConfigForm.attempt_timeout_seconds" type="number" min="5" max="60" class="input input-sm w-full font-mono pr-7 text-xs" />
                <span class="absolute right-2 top-2 text-[11px] text-gray-400">秒</span>
              </div>
              <p class="mt-1 text-[11px] text-gray-400">握手等待上限 (5~60s)</p>
            </div>
            <div>
              <label class="block text-xs font-medium text-gray-700 dark:text-gray-300">提前补票阈值</label>
              <div class="relative mt-1">
                <input v-model.number="autoConfigForm.refresh_before_seconds" type="number" min="60" max="1800" class="input input-sm w-full font-mono pr-7 text-xs" />
                <span class="absolute right-2 top-2 text-[11px] text-gray-400">秒</span>
              </div>
              <p class="mt-1 text-[11px] text-gray-400">到期前提前秒数 (默认10m)</p>
            </div>
          </div>
        </div>

        <!-- 🎯 手动打票专属控制台卡片 -->
        <div class="card overflow-hidden border border-gray-200 dark:border-dark-700 bg-white dark:bg-dark-800 rounded-xl shadow-sm">
          <div class="px-4 py-3 bg-gray-50/50 dark:bg-dark-700/50 border-b border-gray-100 dark:border-dark-700 flex justify-between items-center">
            <div class="flex items-center gap-2">
              <span class="font-semibold text-gray-900 dark:text-white text-sm">🎯 手动打票</span>
              <span class="text-xs bg-emerald-50 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300 px-2 py-0.5 rounded border border-emerald-200 dark:border-emerald-800">单号直通</span>
              <span class="text-xs text-gray-400">绕开全局排队，自由调频、单号定向打票</span>
            </div>
            <span class="text-xs text-gray-400 font-mono">出口: {{ snapshot.sidecar.group || 'CODEX-ROTATE' }}</span>
          </div>

          <div class="grid grid-cols-1 lg:grid-cols-12">
            <!-- 左侧参数设置区 (5 列) -->
            <div class="lg:col-span-5 p-4 border-r border-gray-100 dark:border-dark-700 flex flex-col gap-3">
              <div>
                <label class="text-xs font-semibold text-gray-700 dark:text-gray-300 flex justify-between">
                  <span>目标账号</span>
                  <span class="text-gray-400 font-normal">支持按 #ID 或名称搜索</span>
                </label>
                <div class="relative mt-1">
                  <input
                    v-model="manualAccountKeyword"
                    type="text"
                    placeholder="输入 #ID 或账号名称搜索..."
                    class="input input-sm w-full text-xs"
                    @focus="manualAccountDropdownOpen = true"
                  />
                  <div v-if="manualAccountDropdownOpen" class="absolute z-20 top-full left-0 right-0 bg-white dark:bg-dark-800 border dark:border-dark-700 rounded-md shadow-lg max-h-48 overflow-y-auto mt-1">
                    <div
                      v-for="acc in filteredManualAccounts"
                      :key="acc.id"
                      class="px-3 py-2 text-xs hover:bg-gray-50 dark:hover:bg-dark-700 cursor-pointer flex justify-between items-center"
                      @click="selectManualAccount(acc)"
                    >
                      <span><strong class="text-gray-500 mr-1">#{{ acc.id }}</strong> {{ acc.name }}</span>
                      <span class="text-[10px] px-1.5 py-0.5 rounded" :class="acc.schedulable ? 'bg-emerald-50 text-emerald-700' : 'bg-rose-50 text-rose-700'">
                        {{ acc.ready_count }}/{{ acc.tickets?.length ?? 0 }}
                      </span>
                    </div>
                  </div>
                </div>
              </div>

              <div>
                <label class="text-xs font-semibold text-gray-700 dark:text-gray-300">打票模型</label>
                <div class="flex gap-2 mt-1">
                  <label v-for="m in availableManualModels" :key="m" class="flex-1 border dark:border-dark-700 rounded px-2 py-1.5 text-xs flex items-center justify-center gap-1.5 cursor-pointer" :class="manualSelectedModels.includes(m) ? 'border-emerald-300 bg-emerald-50 text-emerald-700 dark:bg-emerald-950/40 font-medium' : 'text-gray-600 dark:text-gray-400'">
                    <input type="checkbox" :value="m" v-model="manualSelectedModels" class="accent-emerald-600" />
                    <span>{{ m }}</span>
                  </label>
                </div>
              </div>

              <div class="grid grid-cols-2 gap-2">
                <div>
                  <label class="text-xs font-semibold text-gray-700 dark:text-gray-300">重试间隔 (1~300s)</label>
                  <div class="relative mt-1">
                    <input v-model.number="manualForm.probe_interval_seconds" type="number" min="1" max="300" class="input input-sm w-full pr-7 text-xs font-mono" />
                    <span class="absolute right-2 top-2 text-[11px] text-gray-400">秒</span>
                  </div>
                  <p class="mt-0.5 text-[10px] text-gray-400">对应自动打票检测周期</p>
                </div>
                <div>
                  <label class="text-xs font-semibold text-gray-700 dark:text-gray-300">429 冷静 (1~60s)</label>
                  <div class="relative mt-1">
                    <input v-model.number="manualForm.rate_limit_cooldown_seconds" type="number" min="1" max="60" class="input input-sm w-full pr-7 text-xs font-mono" />
                    <span class="absolute right-2 top-2 text-[11px] text-gray-400">秒</span>
                  </div>
                  <p class="mt-0.5 text-[10px] text-gray-400">对应自动打票失败冷却</p>
                </div>
              </div>

              <div class="grid grid-cols-2 gap-2">
                <div>
                  <label class="text-xs font-semibold text-gray-700 dark:text-gray-300">最大尝试</label>
                  <input v-model.number="manualForm.max_attempts" type="number" min="1" max="100" class="input input-sm w-full mt-1 text-xs font-mono" />
                </div>
                <div>
                  <label class="text-xs font-semibold text-gray-700 dark:text-gray-300">换节点规则</label>
                  <select v-model="manualForm.node_switch_rule" class="input input-sm w-full mt-1 text-xs">
                    <option value="312_or_2fail">命中 312 或连败2次切</option>
                    <option value="every_request">每发必切</option>
                    <option value="312_only">仅 312 切</option>
                    <option value="never">固定当前出口</option>
                  </select>
                </div>
              </div>

              <div class="flex items-center gap-2 mt-1">
                <input type="checkbox" id="stopOnSuccess" v-model="manualForm.stop_on_success" class="accent-emerald-600" />
                <label for="stopOnSuccess" class="text-xs text-gray-600 dark:text-gray-300 select-none cursor-pointer">出票即停（成功获取合规门票并入库后自动终止）</label>
              </div>

              <div class="flex gap-2 pt-2 border-t border-dashed dark:border-dark-700">
                <button v-if="!manualHarvesting" type="button" class="btn btn-primary btn-sm flex-1 bg-emerald-600 hover:bg-emerald-700 text-white" :disabled="!selectedManualAccount" @click="startManualHarvest">
                  ▶ 开始打票
                </button>
                <button v-else type="button" class="btn btn-danger btn-sm flex-1 bg-rose-600 hover:bg-rose-700 text-white" @click="stopManualHarvest">
                  ⏹ 停止打票
                </button>
                <button type="button" class="btn btn-secondary btn-sm" @click="manualLogs = []">清空日志</button>
              </div>
            </div>

            <!-- 右侧实时监控与流式日志 (7 列) -->
            <div class="lg:col-span-7 flex flex-col bg-gray-50/30 dark:bg-dark-900/30">
              <div class="px-4 py-2 border-b border-gray-100 dark:border-dark-700 flex justify-between text-xs bg-white dark:bg-dark-800">
                <div><span class="text-gray-400">状态:</span> <strong :class="manualStatusColor">{{ manualStatusText }}</strong></div>
                <div><span class="text-gray-400">尝试进度:</span> <span class="font-mono font-semibold">{{ manualProgressText }}</span></div>
                <div><span class="text-gray-400">当前节点:</span> <span class="font-mono text-primary-600">{{ manualCurrentNode || '-' }}</span></div>
                <div><span class="text-gray-400">入库门票:</span> <strong class="text-emerald-600 font-mono">{{ manualTicketsStoredCount }} 张</strong></div>
              </div>

              <div class="p-3 font-mono text-xs overflow-y-auto max-h-56 flex flex-col gap-1 text-gray-700 dark:text-gray-300">
                <div v-for="(log, idx) in manualLogs" :key="idx" class="flex items-start gap-2">
                  <span class="text-gray-400 text-[10px]">{{ log.time }}</span>
                  <span class="px-1 py-0.5 rounded text-[10px] font-semibold uppercase" :class="logTagClass(log.type)">{{ log.type }}</span>
                  <span class="break-all" v-html="log.text"></span>
                </div>
                <div v-if="!manualLogs.length" class="text-gray-400 italic text-center py-8">点击【开始打票】发起单号定向探针...</div>
              </div>
            </div>
          </div>
        </div>
          <div class="card p-5 xl:col-span-2">
            <h2 class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.harvestFlow.accounts') }}</h2>
            <div v-if="!snapshot.accounts?.length" class="text-sm text-gray-500">{{ t('admin.harvestFlow.noAccounts') }}</div>
            <div v-else class="space-y-3">
              <div v-for="account in snapshot.accounts" :key="account.id" class="rounded-2xl border border-gray-100 p-4 dark:border-dark-700">
                <div class="mb-3 flex items-center justify-between gap-2">
                  <div>
                    <p class="font-medium text-gray-900 dark:text-white">{{ account.name || `#${account.id}` }}</p>
                    <p class="text-xs text-gray-400">
                      {{ account.status }} · {{ account.schedulable ? t('admin.harvestFlow.schedulable') : t('admin.harvestFlow.unschedulable') }}
                      <span v-if="account.skip_harvest"> · {{ t('admin.harvestFlow.skipHarvestBadge') }}</span>
                      <span v-else-if="account.in_scope"> · {{ t('admin.harvestFlow.harvestAccountBadge') }}</span>
                    </p>
                    <p v-if="account.skip_harvest" class="mt-1 text-[11px] text-amber-600 dark:text-amber-400">{{ t('admin.harvestFlow.skipHarvestHint') }}</p>
                  </div>
                  <div class="flex items-center gap-2">
                    <button
                      type="button"
                      class="btn btn-secondary btn-sm"
                      :disabled="!!skipSaving[account.id]"
                      @click="toggleSkipHarvest(account)"
                    >
                      {{ account.skip_harvest ? t('admin.harvestFlow.enableHarvest') : t('admin.harvestFlow.skipHarvest') }}
                    </button>
                    <span class="text-xs text-gray-400">{{ account.ready_count }}/{{ account.tickets?.length ?? 0 }}</span>
                  </div>
                </div>
                <div class="flex flex-wrap gap-2">
                  <span
                    v-for="ticket in account.tickets"
                    :key="ticket.model"
                    class="rounded-2xl px-2.5 py-1.5 text-xs"
                    :class="ticket.ready ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300' : ticket.blocked ? 'bg-rose-50 text-rose-700 dark:bg-rose-900/30 dark:text-rose-300' : 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'"
                  >
                    <span class="font-medium">{{ ticket.model }}</span>
                    <span v-if="ticket.ready && account.skip_harvest"> · {{ t('admin.harvestFlow.leftoverUnused') }}</span>
                    <span v-else-if="ticket.ready"> · {{ t('admin.harvestFlow.remaining', { time: formatRemaining(ticket.remaining_seconds) }) }}</span>
                    <span v-else-if="ticket.blocked"> · {{ t('admin.harvestFlow.blocked') }}</span>
                    <span v-else> · {{ t('admin.harvestFlow.missing') }}</span>
                    <span v-if="ticket.length" class="block font-mono text-[11px] opacity-80">{{ ticket.length }}B</span>
                    <span v-if="ticket.probe?.result" class="block text-[11px] opacity-80">
                      {{ resultLabel(ticket.probe.result) }}
                      <span v-if="ticket.probe.http_status"> · HTTP {{ ticket.probe.http_status }}</span>
                    </span>
                  </span>
                </div>
              </div>
            </div>
          </div>

          <div class="card p-5 xl:col-span-3">
            <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
              <h2 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.harvestFlow.events') }}</h2>
              <div class="flex flex-wrap gap-1">
                <button
                  v-for="filter in eventFilters"
                  :key="filter"
                  type="button"
                  class="rounded-full px-2.5 py-1 text-[11px]"
                  :class="eventFilter === filter ? 'bg-primary-600 text-white' : 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'"
                  @click="eventFilter = filter"
                >
                  {{ filter === 'all' ? t('admin.harvestFlow.filterAll') : t(`admin.harvestFlow.stages.${filter}`) }}
                </button>
              </div>
            </div>
            <div v-if="!snapshot.events?.length" class="text-sm text-gray-500">{{ t('admin.harvestFlow.noEvents') }}</div>
            <ol v-else class="max-h-[560px] space-y-2 overflow-auto pr-1">
              <li
                v-for="event in filteredEvents"
                :key="event.id"
                class="flex gap-3 rounded-xl border border-gray-100 p-3 dark:border-dark-700"
              >
                <span class="mt-1 h-2.5 w-2.5 flex-shrink-0 rounded-full" :class="eventDotClass(event.kind)" />
                <div class="min-w-0 flex-1">
                  <div class="flex flex-wrap items-center justify-between gap-2">
                    <p class="text-sm font-medium text-gray-900 dark:text-white">
                      {{ kindLabel(event.kind) }}
                      <span v-if="event.model" class="ml-1 font-mono text-xs text-gray-500">{{ event.model }}</span>
                    </p>
                    <span class="text-[11px] text-gray-400">{{ formatClock(event.at) }}</span>
                  </div>
                  <p class="mt-1 break-all font-mono text-xs text-gray-500 dark:text-gray-400">
                    <span v-if="event.account_name">{{ event.account_name }}</span>
                    <span v-if="event.node"> · {{ event.node }}</span>
                    <span v-if="event.length"> · {{ event.length }}/{{ event.blocks || '-' }}</span>
                    <span v-if="event.expected_length && event.length && event.length !== event.expected_length">
                      · {{ t('admin.harvestFlow.wantShape', { length: event.expected_length, blocks: event.expected_blocks || '-' }) }}
                    </span>
                    <span v-if="event.http_status"> · HTTP {{ event.http_status }}</span>
                    <span v-if="event.standby"> · {{ t('admin.harvestFlow.standby') }}</span>
                  </p>
                  <p v-if="event.reason || event.detail || event.result" class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                    {{ reasonLabel(event.reason) || resultLabel(event.result) || event.detail || event.result }}
                  </p>
                </div>
              </li>
            </ol>
          </div>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'
import { getCodexHarvestFlow, updateCodexSkipHarvest, updateCodexHarvestConfig, type CodexHarvestFlowAccount, type CodexHarvestFlowEvent, type CodexHarvestFlowSnapshot, type CodexHarvestFlowStage } from '@/api/admin/accounts'

const { t } = useI18n()
const snapshot = ref<CodexHarvestFlowSnapshot | null>(null)
const loading = ref(false)
const refreshing = ref(false)
const autoRefresh = ref(true)
const errorMessage = ref('')
const lastUpdated = ref('')
const eventFilter = ref<'all' | 'node' | 'probe' | 'shape' | 'ticket' | 'select'>('all')
const eventFilters = ['all', 'node', 'probe', 'shape', 'ticket', 'select'] as const
const skipSaving = ref<Record<number, boolean>>({})
let timer: number | undefined

// ⚙️ 自动打票参数配置状态
const autoConfigOpen = ref(false)
const savingAutoConfig = ref(false)
const autoConfigForm = ref({
  probe_interval_seconds: 180,
  max_probes_per_round: 6,
  cooldown_seconds: 180,
  attempt_timeout_seconds: 25,
  refresh_before_seconds: 600,
})

function toggleAutoConfigPanel() {
  autoConfigOpen.value = !autoConfigOpen.value
  if (autoConfigOpen.value && snapshot.value?.harvest) {
    autoConfigForm.value.probe_interval_seconds = snapshot.value.harvest.probe_interval_seconds || 180
    autoConfigForm.value.max_probes_per_round = snapshot.value.harvest.max_probes_per_round || 6
    autoConfigForm.value.cooldown_seconds = snapshot.value.harvest.cooldown_seconds || 180
    autoConfigForm.value.attempt_timeout_seconds = 25
    autoConfigForm.value.refresh_before_seconds = 600
  }
}

async function saveAutoConfig() {
  savingAutoConfig.value = true
  try {
    await updateCodexHarvestConfig(autoConfigForm.value)
    await fetchFlow()
    autoConfigOpen.value = false
  } catch (err: any) {
    errorMessage.value = err?.response?.data?.message || '保存自动打票参数失败'
  } finally {
    savingAutoConfig.value = false
  }
}

function resetAutoConfig() {
  autoConfigForm.value = {
    probe_interval_seconds: 180,
    max_probes_per_round: 6,
    cooldown_seconds: 180,
    attempt_timeout_seconds: 25,
    refresh_before_seconds: 600,
  }
}

// 🎯 手动打票状态与交互
const manualAccountKeyword = ref('')
const manualAccountDropdownOpen = ref(false)
const selectedManualAccount = ref<CodexHarvestFlowAccount | null>(null)
const availableManualModels = computed(() => snapshot.value?.harvest?.models || ['gpt-6-astra', 'gpt-5.6-sol'])
const manualSelectedModels = ref<string[]>(['gpt-6-astra', 'gpt-5.6-sol'])
const manualHarvesting = ref(false)
const manualStatusText = ref('待命中')
const manualStatusColor = ref('text-gray-400')
const manualProgressText = ref('0 / 20')
const manualCurrentNode = ref('')
const manualTicketsStoredCount = ref(0)
const manualLogs = ref<Array<{ time: string; type: string; text: string }>>([])
let manualEventSource: EventSource | null = null

const filteredManualAccounts = computed(() => {
  const list = snapshot.value?.accounts || []
  const kw = manualAccountKeyword.value.toLowerCase().trim().replace('#', '')
  if (!kw) return list
  return list.filter(a => String(a.id).includes(kw) || (a.name && a.name.toLowerCase().includes(kw)))
})

function selectManualAccount(acc: CodexHarvestFlowAccount) {
  selectedManualAccount.value = acc
  manualAccountKeyword.value = `#${acc.id} · ${acc.name || 'Account'}`
  manualAccountDropdownOpen.value = false
  addManualLog('SELECT', `已选定目标账号: #${acc.id} ${acc.name}`)
}

const manualForm = ref({
  probe_interval_seconds: 10,
  rate_limit_cooldown_seconds: 30,
  max_attempts: 20,
  node_switch_rule: '312_or_2fail',
  stop_on_success: true,
})

function addManualLog(type: string, text: string) {
  const time = new Date().toTimeString().split(' ')[0]
  manualLogs.value.unshift({ time, type, text })
  if (manualLogs.value.length > 80) manualLogs.value.pop()
}

function logTagClass(type: string) {
  switch (type) {
    case 'HIT':
      return 'bg-emerald-100 text-emerald-800 dark:bg-emerald-950/60 dark:text-emerald-300'
    case 'MISS':
      return 'bg-rose-100 text-rose-800 dark:bg-rose-950/60 dark:text-rose-300'
    case '429':
      return 'bg-amber-100 text-amber-800 dark:bg-amber-950/60 dark:text-amber-300'
    case 'SWITCH':
      return 'bg-sky-100 text-sky-800 dark:bg-sky-950/60 dark:text-sky-300'
    default:
      return 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-gray-300'
  }
}

async function startManualHarvest() {
  if (!selectedManualAccount.value) return
  manualHarvesting.value = true
  manualStatusText.value = '打票中...'
  manualStatusColor.value = 'text-amber-500'
  manualProgressText.value = `0 / ${manualForm.value.max_attempts}`
  manualTicketsStoredCount.value = 0
  addManualLog('START', `向账号 #${selectedManualAccount.value.id} 发起定向打票...`)

  try {
    const res = await fetch(`/api/v1/admin/accounts/${selectedManualAccount.value.id}/manual-harvest`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        models: manualSelectedModels.value,
        probe_interval_seconds: manualForm.value.probe_interval_seconds,
        rate_limit_cooldown_seconds: manualForm.value.rate_limit_cooldown_seconds,
        max_attempts: manualForm.value.max_attempts,
        node_switch_rule: manualForm.value.node_switch_rule,
        stop_on_success: manualForm.value.stop_on_success,
      })
    })

    if (!res.ok || !res.body) {
      throw new Error(`HTTP ${res.status}`)
    }

    const reader = res.body.getReader()
    const decoder = new TextDecoder()
    let buffer = ''

    while (true) {
      const { done, value } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true })
      const lines = buffer.split('\n\n')
      buffer = lines.pop() || ''

      for (const line of lines) {
        if (line.startsWith('data: ')) {
          try {
            const data = JSON.parse(line.slice(6))
            manualProgressText.value = `${data.attempt} / ${data.max_attempts}`
            if (data.node) manualCurrentNode.value = data.node
            if (data.tickets_stored) manualTicketsStoredCount.value = data.tickets_stored

            if (data.result === 'hit') {
              addManualLog('HIT', data.message)
            } else if (data.result === 'rate_limited') {
              addManualLog('429', data.message)
            } else if (data.result === 'miss_degraded') {
              addManualLog('MISS', data.message)
            } else {
              addManualLog('INFO', data.message)
            }

            if (data.done) {
              manualHarvesting.value = false
              manualStatusText.value = data.tickets_stored > 0 ? '打票成功' : '任务结束'
              manualStatusColor.value = data.tickets_stored > 0 ? 'text-emerald-500' : 'text-gray-400'
              fetchFlow()
            }
          } catch (_) {}
        }
      }
    }
  } catch (err: any) {
    addManualLog('ERROR', `连接断开或打票中断: ${err.message}`)
  } finally {
    manualHarvesting.value = false
  }
}

function stopManualHarvest() {
  manualHarvesting.value = false
  manualStatusText.value = '已停止'
  manualStatusColor.value = 'text-gray-400'
  addManualLog('STOP', '管理员手动终止了打票任务。')
}

const filteredEvents = computed(() => {
  const events = snapshot.value?.events || []
  if (eventFilter.value === 'all') return events
  if (eventFilter.value === 'shape') {
    return events.filter((event: CodexHarvestFlowEvent) => event.stage === 'probe' || event.kind === 'accept' || event.kind === 'reject')
  }
  return events.filter((event: CodexHarvestFlowEvent) => event.stage === eventFilter.value)
})

const stageIcons: Record<string, 'globe' | 'bolt' | 'beaker' | 'key' | 'user'> = {
  node: 'globe',
  probe: 'bolt',
  shape: 'beaker',
  ticket: 'key',
  select: 'user'
}

function stageIcon(id: string) {
  return stageIcons[id] || 'bolt'
}

function stageCardClass(status: string) {
  switch (status) {
    case 'ok':
      return 'border-emerald-200 bg-emerald-50/60 dark:border-emerald-900/40 dark:bg-emerald-950/20'
    case 'warn':
      return 'border-amber-200 bg-amber-50/70 dark:border-amber-900/40 dark:bg-amber-950/20'
    case 'fail':
      return 'border-rose-200 bg-rose-50/70 dark:border-rose-900/40 dark:bg-rose-950/20'
    default:
      return 'border-gray-100 bg-gray-50/80 dark:border-dark-700 dark:bg-dark-800/40'
  }
}

function stageIconClass(status: string) {
  switch (status) {
    case 'ok':
      return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300'
    case 'warn':
      return 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300'
    case 'fail':
      return 'bg-rose-100 text-rose-700 dark:bg-rose-900/40 dark:text-rose-300'
    default:
      return 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-gray-300'
  }
}

function stageTextClass(status: string) {
  switch (status) {
    case 'ok':
      return 'text-emerald-600 dark:text-emerald-400'
    case 'warn':
      return 'text-amber-600 dark:text-amber-400'
    case 'fail':
      return 'text-rose-600 dark:text-rose-400'
    default:
      return 'text-gray-400'
  }
}

function eventDotClass(kind: string) {
  if (kind === 'probe_hit' || kind === 'accept' || kind === 'selected' || kind === 'rotate') {
    return 'bg-emerald-500'
  }
  if (kind === 'skip' || kind === 'probe_miss') {
    return 'bg-amber-500'
  }
  return 'bg-rose-500'
}

function lookupLabel(prefix: string, value?: string) {
  if (!value) return ''
  const key = `${prefix}.${value}`
  const label = t(key)
  return label === key ? value : label
}

function statusLabel(status: string) {
  return lookupLabel('admin.harvestFlow.status', status)
}

function kindLabel(kind: string) {
  return lookupLabel('admin.harvestFlow.kinds', kind)
}

function reasonLabel(reason?: string) {
  return lookupLabel('admin.harvestFlow.reasons', reason)
}

function resultLabel(result?: string) {
  return lookupLabel('admin.harvestFlow.results', result)
}

function scopeLabel(mode?: string, policy?: string, groupIds?: number[]) {
  const scope = mode === 'selected'
    ? t('admin.harvestFlow.scopeSelected', { n: groupIds?.length || 0 })
    : t('admin.harvestFlow.scopeAll')
  const accountPolicy = policy === 'prioritize_schedulable'
    ? t('admin.harvestFlow.policyPrioritize')
    : t('admin.harvestFlow.policySchedulable')
  return `${scope} · ${accountPolicy}`
}

function stageDetail(stage: CodexHarvestFlowStage) {
  switch (stage.id) {
    case 'node':
      if (snapshot.value?.sidecar.now) return snapshot.value.sidecar.now
      if (snapshot.value?.sidecar.reachable) {
        return t('admin.harvestFlow.poolOnline', { n: snapshot.value.sidecar.all_count || 0 })
      }
      return t('admin.harvestFlow.waitingSidecar')
    case 'probe':
      if (stage.status === 'idle') return t('admin.harvestFlow.idleProbe')
      return resultLabel(stage.detail) || stage.node || stage.detail || t('admin.harvestFlow.idleProbe')
    case 'shape':
      if (stage.status === 'idle') return t('admin.harvestFlow.idleShape')
      if (stage.status === 'ok') {
        return t('admin.harvestFlow.shapeOk', { length: stage.length || 0, blocks: stage.blocks || 0 })
      }
      if (stage.status === 'warn' || stage.detail === 'no ticket body') {
        return t('admin.harvestFlow.shapeNoBody')
      }
      return t('admin.harvestFlow.shapeBad', {
        length: stage.length || 0,
        blocks: stage.blocks || 0,
        expected_length: stage.expected_length || 292,
        expected_blocks: stage.expected_blocks || 10
      })
    case 'ticket':
      if (stage.status === 'idle') return t('admin.harvestFlow.idleTicket')
      if (stage.status === 'fail') {
        return snapshot.value?.counts.tickets_blocked
          ? t('admin.harvestFlow.ticketsPausedCount', { n: snapshot.value.counts.tickets_blocked })
          : t('admin.harvestFlow.ticketRejected')
      }
      if (stage.detail === 'standby stored') return t('admin.harvestFlow.ticketStandby')
      return t('admin.harvestFlow.ticketsReadyCount', { n: snapshot.value?.counts.tickets_ready ?? 0 })
    case 'select':
      if (stage.status === 'idle') return t('admin.harvestFlow.idleSelect')
      return reasonLabel(stage.detail) || resultLabel(stage.detail) || stage.detail || t('admin.harvestFlow.idleSelect')
    default:
      return stage.detail || '—'
  }
}

function formatClock(value?: string) {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleTimeString()
}

function formatRemaining(seconds: number) {
  const total = Math.max(0, seconds)
  const hours = Math.floor(total / 3600)
  const minutes = Math.floor((total % 3600) / 60)
  if (hours > 0) return `${hours}h ${minutes}m`
  if (minutes > 0) return `${minutes}m`
  return `${total}s`
}

async function fetchFlow() {
  if (snapshot.value) {
    refreshing.value = true
  } else {
    loading.value = true
  }
  errorMessage.value = ''
  try {
    snapshot.value = await getCodexHarvestFlow()
    lastUpdated.value = new Date().toLocaleTimeString()
  } catch (error) {
    const err = error as { message?: string }
    errorMessage.value = err?.message || String(error)
  } finally {
    loading.value = false
    refreshing.value = false
  }
}

async function toggleSkipHarvest(account: CodexHarvestFlowAccount) {
  skipSaving.value = { ...skipSaving.value, [account.id]: true }
  errorMessage.value = ''
  try {
    await updateCodexSkipHarvest(account.id, !account.skip_harvest)
    await fetchFlow()
  } catch (error) {
    const err = error as { message?: string }
    errorMessage.value = err?.message || String(error)
  } finally {
    skipSaving.value = { ...skipSaving.value, [account.id]: false }
  }
}

onMounted(() => {
  void fetchFlow()
  timer = window.setInterval(() => {
    if (autoRefresh.value) void fetchFlow()
  }, 5000)
})

onBeforeUnmount(() => {
  if (timer) window.clearInterval(timer)
})
</script>
