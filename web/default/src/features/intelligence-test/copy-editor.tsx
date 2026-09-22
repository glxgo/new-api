import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Accordion } from '@/components/ui/accordion'
import { Button } from '@/components/ui/button'
import {
  Field,
  FieldLabel,
  FieldDescription,
  FieldGroup,
} from '@/components/ui/field'
import { Textarea } from '@/components/ui/textarea'
import { SettingsAccordion } from '@/features/system-settings/components/settings-accordion'

const sections = [
  {
    id: 'overview',
    title: 'User page → Overview',
    keys: ['page_title', 'page_intro'],
  },
  {
    id: 'works',
    title: 'User page → Works and empty states',
    keys: ['gallery_title', 'empty_text'],
  },
  {
    id: 'method',
    title: 'User page → Test methodology',
    keys: [
      'method_intro',
      'method_scope_title',
      'method_evidence_title',
      'method_selection_title',
      'method_reading_title',
      'method_trace_title',
      'method_limits_title',
    ],
  },
] as const

export function CopyEditor(props: {
  slots: ReadonlyArray<readonly [string, string, string]>
  value: Record<string, string>
  saved: Record<string, string>
  disabled: boolean
  onChange: (value: Record<string, string>) => void
}) {
  const { t } = useTranslation()
  const [open, setOpen] = useState<string[]>([])
  return (
    <section className='rounded-lg border px-4'>
      <div className='flex flex-wrap items-center justify-between gap-2 border-b py-3'>
        <h4 className='text-sm font-semibold'>{t('Copy management')}</h4>
        <div className='flex gap-1'>
          <Button
            size='sm'
            variant='ghost'
            disabled={open.length === sections.length}
            onClick={() => setOpen(sections.map((s) => s.id))}
          >
            {t('Expand all')}
          </Button>
          <Button
            size='sm'
            variant='ghost'
            disabled={!open.length}
            onClick={() => setOpen([])}
          >
            {t('Collapse all')}
          </Button>
        </div>
      </div>
      <Accordion multiple value={open} onValueChange={setOpen}>
        {sections.map((section) => {
          const slots = props.slots.filter(([key]) =>
            (section.keys as readonly string[]).includes(key)
          )
          const customized = slots.filter(([key]) => !!props.value[key]).length
          const changed = slots.some(
            ([key]) => (props.value[key] || '') !== (props.saved[key] || '')
          )
          const title = `${t(section.title)} · ${t('{{count}} customized fields', { count: customized })}${changed ? ` · ${t('Unsaved changes')}` : ''}`
          return (
            <SettingsAccordion
              key={section.id}
              value={section.id}
              title={title}
            >
              <FieldGroup>
                {slots.map(([key, label, location]) => (
                  <Field key={key}>
                    <FieldLabel htmlFor={`iq-copy-${key}`}>
                      {t(label)}
                    </FieldLabel>
                    <Textarea
                      id={`iq-copy-${key}`}
                      value={props.value[key] || ''}
                      maxLength={500}
                      disabled={props.disabled}
                      onChange={(event) => {
                        const next = { ...props.value }
                        if (event.target.value) next[key] = event.target.value
                        else delete next[key]
                        props.onChange(next)
                      }}
                    />
                    <FieldDescription>
                      {t(location)} ·{' '}
                      {t('Leave empty to use the default text.')}
                    </FieldDescription>
                  </Field>
                ))}
              </FieldGroup>
            </SettingsAccordion>
          )
        })}
      </Accordion>
    </section>
  )
}
