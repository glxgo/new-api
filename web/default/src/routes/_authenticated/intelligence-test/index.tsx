import { createFileRoute } from '@tanstack/react-router'
import { PelicanArchivePage } from '@/features/pelican-archive'

export const Route = createFileRoute('/_authenticated/intelligence-test/')({
  component: PelicanArchivePage,
})
