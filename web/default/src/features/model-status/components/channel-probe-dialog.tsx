/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Activity,
  CheckCircle2,
  CircleDashed,
  Loader2,
  RefreshCw,
  TriangleAlert,
} from 'lucide-react'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Dialog } from '@/components/dialog'
import {
  getChannelProbeStatus,
  probeChannelNow,
  type ChannelProbeAdminState,
} from '../api'
import { formatProbeTime, probeErrorLabel } from '../probe'

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
}

function stateLabel(state: ChannelProbeAdminState) {
  if (!state.last_probe_ts) return '尚未探测'
  if (state.status === 'healthy') return '正常'
  if (state.status === 'unhealthy') return '连续失败'
  if (state.status === 'degraded') return '单次波动'
  return '恢复确认中'
}

function StateIcon({ state }: { state: ChannelProbeAdminState }) {
  if (!state.last_probe_ts || state.status === 'checking') {
    return <CircleDashed className='size-4' />
  }
  if (state.status === 'healthy') return <CheckCircle2 className='size-4' />
  return <TriangleAlert className='size-4' />
}

export function ChannelProbeDialog({ open, onOpenChange }: Props) {
  const queryClient = useQueryClient()
  const statusQuery = useQuery({
    queryKey: ['channel-probe-status'],
    queryFn: getChannelProbeStatus,
    enabled: open,
    staleTime: 15_000,
    refetchInterval: open ? 30_000 : false,
  })
  const probeMutation = useMutation({
    mutationFn: probeChannelNow,
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['channel-probe-status'] }),
        queryClient.invalidateQueries({
          queryKey: ['perf-metrics-group-summary-model-status'],
        }),
      ])
      toast.success('探测完成，未修改渠道启用状态')
    },
    onError: () => toast.error('探测请求失败，请查看服务端日志'),
  })
  const items = statusQuery.data?.data ?? []
  const enabledItems = items.filter((item) => item.channel_status === 1)
  const healthyCount = enabledItems.filter(
    (item) => item.status === 'healthy' && item.last_probe_ts
  ).length

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title='渠道金丝雀诊断'
      description='独立低频探测，仅记录状态，不会自动禁用渠道，也不会改变路由优先级。'
      contentClassName='sm:max-w-4xl'
      bodyClassName='space-y-4'
    >
      <div className='grid grid-cols-3 divide-x rounded-xl border'>
        <div className='px-3 py-3 text-center'>
          <p className='text-muted-foreground text-[10px]'>已启用渠道</p>
          <p className='mt-1 font-mono text-lg font-semibold'>
            {enabledItems.length}
          </p>
        </div>
        <div className='px-3 py-3 text-center'>
          <p className='text-muted-foreground text-[10px]'>探测正常</p>
          <p className='mt-1 font-mono text-lg font-semibold'>{healthyCount}</p>
        </div>
        <div className='px-3 py-3 text-center'>
          <p className='text-muted-foreground text-[10px]'>观察或异常</p>
          <p className='mt-1 font-mono text-lg font-semibold'>
            {Math.max(0, enabledItems.length - healthyCount)}
          </p>
        </div>
      </div>

      {statusQuery.isLoading ? (
        <div className='space-y-2'>
          {Array.from({ length: 4 }).map((_, index) => (
            <Skeleton key={index} className='h-28 rounded-xl' />
          ))}
        </div>
      ) : statusQuery.isError ? (
        <div className='text-muted-foreground rounded-xl border border-dashed py-10 text-center text-sm'>
          无法读取探测状态
        </div>
      ) : (
        <div className='space-y-2'>
          {items.map((item) => {
            const isRunning =
              probeMutation.isPending &&
              probeMutation.variables === item.channel_id
            return (
              <article
                key={item.channel_id}
                className={cn(
                  'bg-muted/10 rounded-xl border p-3.5',
                  item.status === 'unhealthy' && item.last_probe_ts
                    ? 'border-destructive/45'
                    : 'border-border/70'
                )}
              >
                <div className='flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between'>
                  <div className='min-w-0'>
                    <div className='flex flex-wrap items-center gap-2'>
                      <span className='bg-background flex size-8 items-center justify-center rounded-lg border'>
                        <StateIcon state={item} />
                      </span>
                      <div>
                        <p className='text-sm font-semibold'>
                          #{item.channel_id} · {item.channel_name}
                        </p>
                        <p className='text-muted-foreground mt-0.5 text-[10px]'>
                          {item.groups.join(' / ') || '未设置分组'} ·{' '}
                          {item.model_name || '等待选择测试模型'}
                        </p>
                      </div>
                      <span className='bg-background rounded-full border px-2 py-0.5 text-[10px] font-medium'>
                        {stateLabel(item)}
                      </span>
                    </div>
                  </div>
                  <Button
                    type='button'
                    size='sm'
                    variant='outline'
                    disabled={
                      item.channel_status !== 1 || probeMutation.isPending
                    }
                    onClick={() => probeMutation.mutate(item.channel_id)}
                  >
                    {isRunning ? (
                      <Loader2 className='size-3.5 animate-spin' />
                    ) : (
                      <RefreshCw className='size-3.5' />
                    )}
                    立即探测
                  </Button>
                </div>

                <div className='mt-3 grid grid-cols-2 gap-2 text-xs sm:grid-cols-4'>
                  <div className='bg-background rounded-lg border px-2.5 py-2'>
                    <p className='text-muted-foreground text-[10px]'>
                      最近探测
                    </p>
                    <p className='mt-1 font-medium'>
                      {formatProbeTime(item.last_probe_ts)}
                    </p>
                  </div>
                  <div className='bg-background rounded-lg border px-2.5 py-2'>
                    <p className='text-muted-foreground text-[10px]'>
                      总延迟 / 首 Token
                    </p>
                    <p className='mt-1 font-mono font-medium'>
                      {item.last_latency_ms ? `${item.last_latency_ms}ms` : '—'}{' '}
                      / {item.has_ttft ? `${item.last_ttft_ms}ms` : '—'}
                    </p>
                  </div>
                  <div className='bg-background rounded-lg border px-2.5 py-2'>
                    <p className='text-muted-foreground text-[10px]'>
                      连续结果
                    </p>
                    <p className='mt-1 font-medium'>
                      成功 {item.consecutive_successes} · 失败{' '}
                      {item.consecutive_failures}
                    </p>
                  </div>
                  <div className='bg-background rounded-lg border px-2.5 py-2'>
                    <p className='text-muted-foreground text-[10px]'>
                      渠道状态
                    </p>
                    <p className='mt-1 font-medium'>
                      {item.channel_status === 1 ? '已启用' : '未启用'}
                    </p>
                  </div>
                </div>

                {item.last_error_category ? (
                  <div className='text-muted-foreground mt-3 flex gap-2 rounded-lg border border-dashed px-3 py-2 text-[11px]'>
                    <Activity className='mt-0.5 size-3.5 shrink-0' />
                    <span className='min-w-0 break-words'>
                      {probeErrorLabel(item.last_error_category)}
                      {item.last_http_status
                        ? ` · HTTP ${item.last_http_status}`
                        : ''}
                      {item.last_error_code ? ` · ${item.last_error_code}` : ''}
                      {item.last_error_message
                        ? ` · ${item.last_error_message}`
                        : ''}
                    </span>
                  </div>
                ) : null}
              </article>
            )
          })}
        </div>
      )}
    </Dialog>
  )
}
