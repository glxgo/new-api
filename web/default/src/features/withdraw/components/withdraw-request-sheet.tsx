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
import { useEffect, useState } from 'react'
import { z } from 'zod'
import { useForm, type Resolver } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatQuota, parseQuotaFromDollars } from '@/lib/format'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import {
  SideDrawerSection,
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import { requestWithdraw } from '../api'
import { WITHDRAW_TYPE, type WithdrawType } from '../types'

interface WithdrawRequestSheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  // 1 = principal (regular user), 2 = dividend (admin/root)
  type: WithdrawType
  // withdrawable balance in quota units
  maxAmount: number
  onSuccess?: () => void
}

export function WithdrawRequestSheet({
  open,
  onOpenChange,
  type,
  maxAmount,
  onSuccess,
}: WithdrawRequestSheetProps) {
  const { t } = useTranslation()
  const isPrincipal = type === WITHDRAW_TYPE.PRINCIPAL
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [wechatPreview, setWechatPreview] = useState<string>('')

  const schema = z
    .object({
      amount: z.coerce.number().positive(t('Amount must be greater than 0')),
      alipay_name: z.string(),
      alipay_account: z.string(),
    })
    .superRefine((data, ctx) => {
      // Principal withdrawals require Alipay payment info; dividend ones don't.
      if (isPrincipal) {
        if (!data.alipay_name.trim()) {
          ctx.addIssue({
            code: z.ZodIssueCode.custom,
            path: ['alipay_name'],
            message: t('Required'),
          })
        }
        if (!data.alipay_account.trim()) {
          ctx.addIssue({
            code: z.ZodIssueCode.custom,
            path: ['alipay_account'],
            message: t('Required'),
          })
        }
      }
    })

  type Values = z.infer<typeof schema>

  const form = useForm<Values>({
    resolver: zodResolver(schema) as unknown as Resolver<Values>,
    defaultValues: {
      amount: 0,
      alipay_name: '',
      alipay_account: '',
    },
  })

  useEffect(() => {
    if (open) {
      form.reset({ amount: 0, alipay_name: '', alipay_account: '' })
      setWechatPreview('')
    }
  }, [open, form])

  const onWechatChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    if (file.size > 5 * 1024 * 1024) {
      toast.error(t('Image too large (max 5MB)'))
      return
    }
    // 压缩微信收款码: canvas 缩放至 max 1200px + jpeg 0.7,
    // 避免 base64 撑爆 DB 的 wechat_qrcode 字段。
    const reader = new FileReader()
    reader.onload = () => {
      const img = new Image()
      img.onload = () => {
        const maxSize = 1200
        let { width, height } = img
        if (width > maxSize || height > maxSize) {
          if (width >= height) {
            height = Math.round((height * maxSize) / width)
            width = maxSize
          } else {
            width = Math.round((width * maxSize) / height)
            height = maxSize
          }
        }
        const canvas = document.createElement('canvas')
        canvas.width = width
        canvas.height = height
        const ctx = canvas.getContext('2d')
        if (!ctx) {
          setWechatPreview(reader.result as string)
          return
        }
        ctx.drawImage(img, 0, 0, width, height)
        setWechatPreview(canvas.toDataURL('image/jpeg', 0.7))
      }
      img.onerror = () => setWechatPreview(reader.result as string)
      img.src = reader.result as string
    }
    reader.onerror = () => toast.error(t('Failed to load image'))
    reader.readAsDataURL(file)
  }

  async function onSubmit(values: Values) {
    const amountQuota = parseQuotaFromDollars(values.amount)
    if (amountQuota <= 0) {
      toast.error(t('Amount must be greater than 0'))
      return
    }
    if (amountQuota > maxAmount) {
      toast.error(t('Amount exceeds withdrawable balance'))
      return
    }
    setIsSubmitting(true)
    try {
      const res = await requestWithdraw({
        type,
        amount: amountQuota,
        alipay_name: isPrincipal ? values.alipay_name : undefined,
        alipay_account: isPrincipal ? values.alipay_account : undefined,
        wechat_qrcode: isPrincipal && wechatPreview ? wechatPreview : undefined,
      })
      if (res.success) {
        toast.success(t('Withdrawal request submitted'))
        onOpenChange(false)
        onSuccess?.()
      } else {
        toast.error(res.message || t('Failed to submit withdrawal request'))
      }
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Sheet
      open={open}
      onOpenChange={(v) => {
        onOpenChange(v)
        if (!v) form.reset()
      }}
    >
      <SheetContent className={sideDrawerContentClassName('sm:max-w-[520px]')}>
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>
            {isPrincipal ? t('Withdraw Principal') : t('Withdraw Dividend')}
          </SheetTitle>
          <SheetDescription>
            {isPrincipal ? (
              <span>
                本金余额可提现，收取{' '}
                <span className='font-semibold text-rose-600'>2.6%</span>{' '}
                手续费（由支付平台收取）。充值 7
                天内的金额处于冻结期，不可提现。
              </span>
            ) : (
              <span>分红余额可提现，超管将线下联系您打款。</span>
            )}
          </SheetDescription>
        </SheetHeader>

        <Form {...form}>
          <form
            id='withdraw-request-form'
            onSubmit={form.handleSubmit(onSubmit)}
            className={sideDrawerFormClassName()}
          >
            <SideDrawerSection>
              <FormField
                control={form.control}
                name='amount'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Withdrawal Amount')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={0}
                        step='0.01'
                        placeholder={t('Withdrawable: {{amount}}', {
                          amount: formatQuota(maxAmount),
                        })}
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Withdrawable balance: {{amount}}', {
                        amount: formatQuota(maxAmount),
                      })}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              {isPrincipal && (
                <>
                  <FormField
                    control={form.control}
                    name='alipay_name'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Alipay Real Name')}</FormLabel>
                        <FormControl>
                          <Input
                            placeholder={t('Alipay account holder name')}
                            {...field}
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='alipay_account'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Alipay Account')}</FormLabel>
                        <FormControl>
                          <Input
                            placeholder={t('Alipay account (mobile or email)')}
                            {...field}
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormItem>
                    <FormLabel>
                      {t('WeChat Payment QR (optional backup)')}
                    </FormLabel>
                    <FormControl>
                      <Input
                        type='file'
                        accept='image/png,image/jpeg'
                        onChange={onWechatChange}
                      />
                    </FormControl>
                    {wechatPreview && (
                      <img
                        src={wechatPreview}
                        alt='wechat-qr'
                        className='mt-2 h-32 w-auto rounded border object-contain'
                      />
                    )}
                    <FormDescription>
                      {t('Used as a backup payment method')}
                    </FormDescription>
                  </FormItem>
                </>
              )}
            </SideDrawerSection>
          </form>
        </Form>

        <SheetFooter className={sideDrawerFooterClassName()}>
          <SheetClose render={<Button variant='outline' />}>
            {t('Close')}
          </SheetClose>
          <Button
            form='withdraw-request-form'
            type='submit'
            disabled={isSubmitting}
          >
            {isSubmitting ? t('Submitting...') : t('Submit Request')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
