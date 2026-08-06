import { useEffect, useState } from 'react'
import { Activity, Link2, Save, Trash2 } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { SectionPageLayout } from '@/components/layout'
import {
  deleteAdminAPIIngressProfile,
  getAdminAPIIngressProfiles,
  saveAdminAPIIngressProfile,
} from './api'
import type { APIIngressProfile } from './api'

const blank: Partial<APIIngressProfile> = {
  code: '',
  display_name: '',
  public_base_url: '',
  network_mode: 'direct',
  description: '',
  multiplier: 1,
  billing_multiplier_ppm: 1000000,
  enabled: true,
  visible: true,
  default: false,
  probe_enabled: true,
  sort_order: 0,
}

export function APIIngress() {
  const [profiles, setProfiles] = useState<APIIngressProfile[]>([])
  const [editing, setEditing] = useState<Partial<APIIngressProfile>>(blank)
  const load = async () => {
    const result = await getAdminAPIIngressProfiles()
    setProfiles(result.data ?? [])
  }
  useEffect(() => {
    void load()
  }, [])
  const update = (field: string, value: string | number | boolean) =>
    setEditing((current) => ({ ...current, [field]: value }))
  const save = async () => {
    try {
      const result = await saveAdminAPIIngressProfile({
        ...editing,
        billing_multiplier_ppm: Math.round(
          Number(editing.multiplier ?? 1) * 1000000
        ),
      })
      if (result.success) {
        toast.success('入口已保存')
        setEditing(blank)
        await load()
      }
    } catch {
      /* interceptor handles error */
    }
  }
  const remove = async (profile: APIIngressProfile) => {
    if (!window.confirm(`确认删除 ${profile.display_name}？`)) return
    const result = await deleteAdminAPIIngressProfile(profile.id)
    if (result.success) {
      toast.success('入口已删除')
      await load()
    }
  }
  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>API 入口与倍率</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='grid gap-5 xl:grid-cols-[minmax(0,1fr)_24rem]'>
          <div className='bg-card overflow-hidden rounded-2xl border'>
            <div className='border-b p-5'>
              <h2 className='font-semibold'>入口列表</h2>
              <p className='text-muted-foreground mt-1 text-xs'>
                由反向代理给请求注入入口编码；这里的倍率只影响用户售价，渠道成本仍按原价记账。
              </p>
            </div>
            {profiles.map((profile) => (
              <div
                key={profile.id}
                className='flex items-center justify-between gap-3 border-b p-5 last:border-b-0'
              >
                <button
                  type='button'
                  className='min-w-0 flex-1 text-left'
                  onClick={() =>
                    setEditing({
                      ...profile,
                      multiplier: profile.billing_multiplier_ppm / 1000000,
                    })
                  }
                >
                  <div className='flex items-center gap-2'>
                    <Link2 className='size-4 text-emerald-600' />
                    <p className='font-medium'>{profile.display_name}</p>
                    {profile.default && (
                      <span className='rounded-full bg-blue-500/10 px-2 py-0.5 text-[11px] text-blue-600'>
                        默认
                      </span>
                    )}
                  </div>
                  <p className='text-muted-foreground mt-1 truncate text-xs'>
                    {profile.public_base_url || '尚未配置公网地址'} · ×
                    {(profile.billing_multiplier_ppm / 1000000).toFixed(3)}
                  </p>
                  <p className='text-muted-foreground mt-1 text-xs'>
                    {profile.description}
                  </p>
                </button>
                <Button
                  variant='ghost'
                  size='icon'
                  disabled={profile.default}
                  onClick={() => remove(profile)}
                >
                  <Trash2 className='size-4' />
                </Button>
              </div>
            ))}
          </div>
          <div className='bg-card rounded-2xl border p-5'>
            <div className='flex items-center gap-2'>
              <Activity className='size-5 text-emerald-600' />
              <h2 className='font-semibold'>
                {editing.id ? '编辑入口' : '新增入口'}
              </h2>
            </div>
            <div className='mt-4 space-y-3'>
              <Input
                placeholder='编码，例如 direct'
                value={editing.code ?? ''}
                onChange={(event) => update('code', event.target.value)}
              />
              <Input
                placeholder='显示名称'
                value={editing.display_name ?? ''}
                onChange={(event) => update('display_name', event.target.value)}
              />
              <Input
                placeholder='公网 Base URL，例如 https://api.example.com'
                value={editing.public_base_url ?? ''}
                onChange={(event) =>
                  update('public_base_url', event.target.value)
                }
              />
              <Input
                placeholder='网络模式：line / direct'
                value={editing.network_mode ?? ''}
                onChange={(event) => update('network_mode', event.target.value)}
              />
              <Textarea
                placeholder='说明'
                value={editing.description ?? ''}
                onChange={(event) => update('description', event.target.value)}
                className='min-h-20'
              />
              <label className='text-xs'>
                <span className='text-muted-foreground mb-1 block'>
                  用户倍率（0.95 = 95 折）
                </span>
                <Input
                  type='number'
                  min='0.01'
                  max='2'
                  step='0.001'
                  value={editing.multiplier ?? 1}
                  onChange={(event) =>
                    update('multiplier', Number(event.target.value))
                  }
                />
              </label>
              <label className='block text-sm'>
                <input
                  className='mr-2'
                  type='checkbox'
                  checked={editing.enabled ?? true}
                  onChange={(event) => update('enabled', event.target.checked)}
                />
                启用入口
              </label>
              <label className='block text-sm'>
                <input
                  className='mr-2'
                  type='checkbox'
                  checked={editing.visible ?? true}
                  onChange={(event) => update('visible', event.target.checked)}
                />
                对用户展示
              </label>
              <label className='block text-sm'>
                <input
                  className='mr-2'
                  type='checkbox'
                  checked={editing.default ?? false}
                  onChange={(event) => update('default', event.target.checked)}
                />
                设为默认入口
              </label>
              <div className='flex gap-2 pt-2'>
                <Button className='flex-1' onClick={save}>
                  <Save className='size-4' />
                  保存
                </Button>
                <Button variant='outline' onClick={() => setEditing(blank)}>
                  清空
                </Button>
              </div>
            </div>
          </div>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
