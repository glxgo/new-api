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
import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { RefreshCw, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import { ROLE } from '@/lib/roles'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  getAccessPolicy,
  getMainlandAllowlists,
  rollbackAccessPolicy,
  revokeMainlandAllowlist,
  updateAccessPolicy,
} from '../api'
import {
  SettingsControlGroup,
  SettingsSwitchField,
} from '../components/settings-form-layout'
import { SettingsSection } from '../components/settings-section'

export function AccessPolicySection() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const isRoot =
    (useAuthStore((state) => state.auth.user?.role) ?? 0) >= ROLE.SUPER_ADMIN
  const [pendingValue, setPendingValue] = useState<boolean | null>(null)
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [rollbackOpen, setRollbackOpen] = useState(false)
  const [revokeId, setRevokeId] = useState<number | null>(null)

  const { data, isLoading } = useQuery({
    queryKey: ['access-policy'],
    queryFn: getAccessPolicy,
  })

  const allowlistsQuery = useQuery({
    queryKey: ['mainland-allowlists'],
    queryFn: () => getMainlandAllowlists(false),
    enabled: isRoot,
  })

  const revokeMutation = useMutation({
    mutationFn: (id: number) => revokeMainlandAllowlist(id),
    onSuccess: (res) => {
      setRevokeId(null)
      if (res.success) {
        toast.success(t('IP whitelist entry deleted'))
        void allowlistsQuery.refetch()
      } else {
        toast.error(res.message || t('Failed to delete IP whitelist entry'))
      }
    },
    onError: (error: Error) => {
      setRevokeId(null)
      toast.error(error.message || t('Failed to delete IP whitelist entry'))
    },
  })

  const saveMutation = useMutation({
    mutationFn: updateAccessPolicy,
    onSuccess: (res) => {
      setConfirmOpen(false)
      if (res.success) {
        toast.success(t('Access policy updated successfully'))
        queryClient.invalidateQueries({ queryKey: ['access-policy'] })
        return
      }
      toast.error(res.message || t('Failed to update access policy'))
    },
    onError: (error: Error) => {
      setConfirmOpen(false)
      toast.error(error.message || t('Failed to update access policy'))
    },
  })

  const rollbackMutation = useMutation({
    mutationFn: rollbackAccessPolicy,
    onSuccess: (res) => {
      setRollbackOpen(false)
      if (res.success) {
        toast.success(t('Access policy rolled back'))
        queryClient.invalidateQueries({ queryKey: ['access-policy'] })
        return
      }
      toast.error(res.message || t('Failed to roll back access policy'))
    },
    onError: (error: Error) => {
      setRollbackOpen(false)
      toast.error(error.message || t('Failed to roll back access policy'))
    },
  })

  if (isLoading || !data?.data) {
    return (
      <SettingsSection title={t('Access Policy')}>
        <div />
      </SettingsSection>
    )
  }

  const policy = data.data
  const current = policy.block_mainland_web_access

  const onSwitchChange = (value: boolean) => {
    setPendingValue(value)
    setConfirmOpen(true)
  }

  const onConfirmSave = () => {
    if (pendingValue === null) return
    saveMutation.mutate({ block_mainland_web_access: pendingValue })
  }

  const onConfirmRollback = () => {
    rollbackMutation.mutate()
  }

  return (
    <SettingsSection title={t('Access Policy')}>
      <SettingsControlGroup>
        <SettingsSwitchField
          checked={current}
          onCheckedChange={onSwitchChange}
          label={t('Restrict mainland China website access')}
          description={t(
            'Block web pages from mainland China IPs with HTTP 451. API access is not affected.'
          )}
        />
        <div className='text-muted-foreground flex flex-col gap-1 pt-2 text-sm'>
          <div>
            {t('GeoIP database')}:{' '}
            {policy.geoip_db_loaded ? t('Loaded') : t('Not loaded')}
            {policy.geoip_db_version ? ` (${policy.geoip_db_version})` : ''}
          </div>
          <div>
            {t('Unknown IP policy')}:{' '}
            {policy.geoip_unknown_policy === 'deny' ? t('Deny') : t('Allow')}
          </div>
          <div>
            {t('Config version')}: {policy.config_version}
          </div>
          <div>
            {t('Blocked requests')}: {policy.stats.block_total} ·{' '}
            {t('Unknown requests')}: {policy.stats.unknown_total} ·{' '}
            {t('Lookup errors')}: {policy.stats.lookup_error_total} ·{' '}
            {t('Decisions')}: {policy.stats.decision_total}
          </div>
        </div>
        <div className='flex gap-2 pt-2'>
          <Button
            variant='outline'
            onClick={() => setRollbackOpen(true)}
            disabled={rollbackMutation.isPending}
          >
            {t('Rollback to previous version')}
          </Button>
        </div>

        <div className='border-border/70 mt-4 space-y-3 border-t pt-4'>
          <div className='flex flex-wrap items-center justify-between gap-2'>
            <div>
              <div className='text-sm font-medium'>
                {t('Mainland IP whitelist management')}
              </div>
              <p className='text-muted-foreground text-xs'>
                {t('View each users IP entries and revoke them individually')}
              </p>
            </div>
            <Button
              variant='outline'
              size='sm'
              onClick={() => void allowlistsQuery.refetch()}
              disabled={allowlistsQuery.isFetching}
            >
              <RefreshCw className='size-3.5' />
              {t('Refresh')}
            </Button>
          </div>
          {!isRoot ? (
            <p className='text-muted-foreground text-sm'>
              {t('Only the super administrator can manage IP whitelists')}
            </p>
          ) : allowlistsQuery.isError ? (
            <p className='text-destructive text-sm'>
              {t('Only the super administrator can manage IP whitelists')}
            </p>
          ) : allowlistsQuery.isLoading ? (
            <p className='text-muted-foreground text-sm'>{t('Loading...')}</p>
          ) : allowlistsQuery.data?.data?.length ? (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('User')}</TableHead>
                  <TableHead>{t('IP address')}</TableHead>
                  <TableHead>{t('Identity')}</TableHead>
                  <TableHead>{t('Source')}</TableHead>
                  <TableHead>{t('Last seen')}</TableHead>
                  <TableHead className='text-right'>{t('Action')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {allowlistsQuery.data.data.map((entry) => (
                  <TableRow key={entry.id}>
                    <TableCell>
                      <div className='font-medium'>
                        {entry.username || `ID ${entry.user_id}`}
                      </div>
                      {entry.email ? (
                        <div className='text-muted-foreground text-xs'>
                          {entry.email}
                        </div>
                      ) : null}
                    </TableCell>
                    <TableCell className='font-mono text-xs'>
                      {entry.ip}
                    </TableCell>
                    <TableCell>
                      <Badge variant='outline'>
                        {entry.identity_type === 'enterprise'
                          ? t('Enterprise')
                          : t('Education')}
                      </Badge>
                    </TableCell>
                    <TableCell className='text-xs'>
                      {entry.source === 'browser_session'
                        ? t('Browser session')
                        : entry.source === 'admin'
                          ? t('Administrator')
                          : t('User request')}
                    </TableCell>
                    <TableCell className='text-muted-foreground text-xs'>
                      {entry.last_seen_at
                        ? new Date(entry.last_seen_at * 1000).toLocaleString()
                        : '-'}
                    </TableCell>
                    <TableCell className='text-right'>
                      <Button
                        variant='destructive'
                        size='sm'
                        onClick={() => setRevokeId(entry.id)}
                        disabled={revokeMutation.isPending}
                      >
                        <Trash2 className='size-3.5' />
                        {t('Delete')}
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          ) : (
            <p className='text-muted-foreground text-sm'>
              {t('No active mainland IP whitelist entries')}
            </p>
          )}
        </div>
      </SettingsControlGroup>

      <AlertDialog open={confirmOpen} onOpenChange={setConfirmOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {pendingValue
                ? t('Enable mainland China restriction?')
                : t('Disable mainland China restriction?')}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {pendingValue
                ? t(
                    'When enabled, mainland China IPs will see the HTTP 451 page on website pages. API, admin pages, and static resources are not affected.'
                  )
                : t(
                    'This will remove the restriction. Make sure this is intended.'
                  )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
            <AlertDialogAction onClick={onConfirmSave}>
              {t('Confirm')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog
        open={revokeId !== null}
        onOpenChange={(open) => {
          if (!open) setRevokeId(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t('Delete this IP whitelist entry?')}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                'The address will be blocked again if mainland restriction is enabled.'
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                if (revokeId !== null) revokeMutation.mutate(revokeId)
              }}
            >
              {t('Delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog open={rollbackOpen} onOpenChange={setRollbackOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t('Restore the previous access policy?')}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                'The current access policy will be replaced by the previous version.'
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
            <AlertDialogAction onClick={onConfirmRollback}>
              {t('Confirm')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </SettingsSection>
  )
}
