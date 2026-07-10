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
import { FileText, Info, Users } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { SectionPageLayout } from '@/components/layout'
import { SubscribersDialog } from './components/subscribers-dialog'
import { SubscriptionIntroDialog } from './components/subscription-intro-dialog'
import { SubscriptionsDialogs } from './components/subscriptions-dialogs'
import { SubscriptionsPrimaryButtons } from './components/subscriptions-primary-buttons'
import {
  SubscriptionsProvider,
  useSubscriptions,
} from './components/subscriptions-provider'
import { SubscriptionsTable } from './components/subscriptions-table'

function SubscriptionsContent() {
  const { t } = useTranslation()
  const { complianceConfirmed } = useSubscriptions()
  const [subscribersOpen, setSubscribersOpen] = useState(false)
  const [introOpen, setIntroOpen] = useState(false)

  return (
    <>
      <SectionPageLayout fixedContent>
        <SectionPageLayout.Title>
          {t('Subscription Management')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <div className='flex items-center gap-2'>
            <Alert variant='default' className='hidden px-3 py-2 sm:flex'>
              <Info className='h-4 w-4' />
              <AlertDescription className='text-xs'>
                {t(
                  'Stripe/Creem requires creating products on the third-party platform and entering the ID'
                )}
              </AlertDescription>
            </Alert>
            <Button
              variant='outline'
              size='sm'
              onClick={() => setIntroOpen(true)}
            >
              <FileText className='h-4 w-4' />
              {t('Plans Intro')}
            </Button>
            <Button
              variant='outline'
              size='sm'
              onClick={() => setSubscribersOpen(true)}
            >
              <Users className='h-4 w-4' />
              {t('Subscribers')}
            </Button>
            <SubscriptionsPrimaryButtons />
          </div>
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <div className='flex h-full min-h-0 flex-col gap-4'>
            {!complianceConfirmed ? (
              <Alert variant='destructive' className='shrink-0'>
                <AlertDescription>
                  {t(
                    'Subscription plan creation and changes are locked until the administrator confirms compliance terms in Payment Gateway settings.'
                  )}
                </AlertDescription>
              </Alert>
            ) : null}
            <div className='min-h-0 flex-1'>
              <SubscriptionsTable />
            </div>
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <SubscriptionsDialogs />
      <SubscriptionIntroDialog open={introOpen} onOpenChange={setIntroOpen} />
      <SubscribersDialog
        open={subscribersOpen}
        onOpenChange={setSubscribersOpen}
      />
    </>
  )
}

export function Subscriptions() {
  return (
    <SubscriptionsProvider>
      <SubscriptionsContent />
    </SubscriptionsProvider>
  )
}
