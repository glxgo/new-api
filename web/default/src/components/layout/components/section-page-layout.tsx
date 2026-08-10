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
import {
  Children,
  isValidElement,
  useState,
  type ReactElement,
  type ReactNode,
} from 'react'
import { cn } from '@/lib/utils'
import { Main } from './main'
import { PageFooterProvider } from './page-footer'

type SlotProps = { children?: ReactNode }

function SectionPageLayoutTitle(_props: SlotProps) {
  return null
}
SectionPageLayoutTitle.displayName = 'SectionPageLayout.Title'

function SectionPageLayoutActions(_props: SlotProps) {
  return null
}
SectionPageLayoutActions.displayName = 'SectionPageLayout.Actions'

function SectionPageLayoutDescription(_props: SlotProps) {
  return null
}
SectionPageLayoutDescription.displayName = 'SectionPageLayout.Description'

function SectionPageLayoutFeatureStrip(_props: SlotProps) {
  return null
}
SectionPageLayoutFeatureStrip.displayName = 'SectionPageLayout.FeatureStrip'

function SectionPageLayoutContent(_props: SlotProps) {
  return null
}
SectionPageLayoutContent.displayName = 'SectionPageLayout.Content'

function SectionPageLayoutBreadcrumb(_props: SlotProps) {
  return null
}
SectionPageLayoutBreadcrumb.displayName = 'SectionPageLayout.Breadcrumb'

export type SectionPageLayoutProps = {
  children: ReactNode
  fixedContent?: boolean
  variant?: 'default' | 'editorial'
  density?: 'default' | 'compact'
}

export function SectionPageLayout(props: SectionPageLayoutProps) {
  const [footerContainer, setFooterContainer] = useState<HTMLDivElement | null>(
    null
  )

  let title: ReactNode = null
  let actions: ReactNode = null
  let description: ReactNode = null
  let featureStrip: ReactNode = null
  let content: ReactNode = null
  let breadcrumb: ReactNode = null

  Children.forEach(props.children, (node) => {
    if (!isValidElement(node)) return
    const child = node as ReactElement<SlotProps>
    if (child.type === SectionPageLayoutTitle) title = child.props.children
    else if (child.type === SectionPageLayoutActions)
      actions = child.props.children
    else if (child.type === SectionPageLayoutDescription)
      description = child.props.children
    else if (child.type === SectionPageLayoutFeatureStrip)
      featureStrip = child.props.children
    else if (child.type === SectionPageLayoutContent)
      content = child.props.children
    else if (child.type === SectionPageLayoutBreadcrumb)
      breadcrumb = child.props.children
  })

  const editorial = props.variant === 'editorial'
  const compact = props.density === 'compact'

  return (
    <PageFooterProvider container={footerContainer}>
      <Main className='overflow-y-auto overscroll-y-contain sm:overflow-hidden'>
        <div
          className={cn(
            'border-border/80 shrink-0 border-b px-[var(--app-page-gutter)]',
            compact
              ? 'pt-3 pb-2.5 sm:pt-4 sm:pb-3'
              : editorial
                ? 'pt-5 pb-4 sm:pt-7 sm:pb-5'
                : 'pt-4 pb-3.5 sm:pt-6 sm:pb-4'
          )}
        >
          {breadcrumb != null && (
            <div className='mb-2 sm:mb-3'>{breadcrumb}</div>
          )}
          <div
            className={cn(
              'flex flex-wrap justify-between gap-x-3 gap-y-3 sm:gap-x-6',
              editorial ? 'items-end' : 'items-center'
            )}
          >
            <div className='min-w-0 flex-1'>
              <h2
                className={cn(
                  'font-serif font-semibold tracking-[-0.025em]',
                  compact ? 'text-xl sm:text-2xl' : 'text-2xl sm:text-3xl',
                  !editorial && 'truncate'
                )}
              >
                {title}
              </h2>
              {description != null && (
                <p
                  className={cn(
                    'text-muted-foreground mt-1 max-w-2xl font-normal',
                    editorial ? 'text-sm leading-6' : 'text-xs'
                  )}
                >
                  {description}
                </p>
              )}
            </div>
            {actions != null && (
              <div className='flex shrink-0 flex-wrap items-center justify-end gap-2 sm:gap-x-4'>
                {actions}
              </div>
            )}
          </div>
          {featureStrip != null && <div>{featureStrip}</div>}
        </div>

        <div
          className={cn(
            'min-h-0 flex-none overflow-visible px-[var(--app-page-gutter)]',
            compact
              ? 'pt-2 pb-2.5 sm:pt-2.5 sm:pb-3'
              : 'pt-3 pb-3 sm:pt-4 sm:pb-5',
            props.fixedContent
              ? 'sm:flex-1 sm:overflow-hidden'
              : 'sm:flex-1 sm:overflow-auto'
          )}
        >
          {content}
        </div>

        <div
          ref={setFooterContainer}
          className='bg-background shrink-0 border-t px-[var(--app-page-gutter)] py-2.5 empty:hidden sm:py-3'
        />
      </Main>
    </PageFooterProvider>
  )
}

SectionPageLayout.Title = SectionPageLayoutTitle
SectionPageLayout.Actions = SectionPageLayoutActions
SectionPageLayout.Description = SectionPageLayoutDescription
SectionPageLayout.FeatureStrip = SectionPageLayoutFeatureStrip
SectionPageLayout.Content = SectionPageLayoutContent
SectionPageLayout.Breadcrumb = SectionPageLayoutBreadcrumb
