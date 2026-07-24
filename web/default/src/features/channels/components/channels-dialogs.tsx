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
import { lazy, Suspense } from 'react'
import { useChannels } from './channels-provider'

const BalanceQueryDialog = lazy(() =>
  import('./dialogs/balance-query-dialog').then((module) => ({
    default: module.BalanceQueryDialog,
  }))
)
const ChannelTestDialog = lazy(() =>
  import('./dialogs/channel-test-dialog').then((module) => ({
    default: module.ChannelTestDialog,
  }))
)
const CopyChannelDialog = lazy(() =>
  import('./dialogs/copy-channel-dialog').then((module) => ({
    default: module.CopyChannelDialog,
  }))
)
const EditTagDialog = lazy(() =>
  import('./dialogs/edit-tag-dialog').then((module) => ({
    default: module.EditTagDialog,
  }))
)
const FetchModelsDialog = lazy(() =>
  import('./dialogs/fetch-models-dialog').then((module) => ({
    default: module.FetchModelsDialog,
  }))
)
const MultiKeyManageDialog = lazy(() =>
  import('./dialogs/multi-key-manage-dialog').then((module) => ({
    default: module.MultiKeyManageDialog,
  }))
)
const OllamaModelsDialog = lazy(() =>
  import('./dialogs/ollama-models-dialog').then((module) => ({
    default: module.OllamaModelsDialog,
  }))
)
const TagBatchEditDialog = lazy(() =>
  import('./dialogs/tag-batch-edit-dialog').then((module) => ({
    default: module.TagBatchEditDialog,
  }))
)
const UpstreamUpdateDialog = lazy(() =>
  import('./dialogs/upstream-update-dialog').then((module) => ({
    default: module.UpstreamUpdateDialog,
  }))
)
const ChannelMutateDrawer = lazy(() =>
  import('./drawers/channel-mutate-drawer').then((module) => ({
    default: module.ChannelMutateDrawer,
  }))
)

type DeferredDialog =
  | 'channel-mutate'
  | 'test-channel'
  | 'balance-query'
  | 'fetch-models'
  | 'ollama-models'
  | 'copy-channel'
  | 'multi-key-manage'
  | 'tag-batch-edit'
  | 'edit-tag'
  | 'upstream-update'

export function ChannelsDialogs() {
  const { open, setOpen, currentRow, upstream } = useChannels()
  const activeDialog: DeferredDialog | null =
    open === 'create-channel' || open === 'update-channel'
      ? 'channel-mutate'
      : open || (upstream.showModal ? 'upstream-update' : null)

  const shouldRender = (dialog: DeferredDialog) => activeDialog === dialog

  return (
    <Suspense fallback={null}>
      {/* Channel Create/Update Drawer */}
      {shouldRender('channel-mutate') && (
        <ChannelMutateDrawer
          open={open === 'create-channel' || open === 'update-channel'}
          onOpenChange={(v) => !v && setOpen(null)}
          currentRow={open === 'update-channel' ? currentRow : null}
        />
      )}

      {/* Test Channel Dialog */}
      {shouldRender('test-channel') && (
        <ChannelTestDialog
          open={open === 'test-channel'}
          onOpenChange={(v) => !v && setOpen(null)}
        />
      )}

      {/* Balance Query Dialog */}
      {shouldRender('balance-query') && (
        <BalanceQueryDialog
          open={open === 'balance-query'}
          onOpenChange={(v) => !v && setOpen(null)}
        />
      )}

      {/* Fetch Models Dialog */}
      {shouldRender('fetch-models') && (
        <FetchModelsDialog
          open={open === 'fetch-models'}
          onOpenChange={(v) => !v && setOpen(null)}
        />
      )}

      {/* Ollama Models Dialog */}
      {shouldRender('ollama-models') && (
        <OllamaModelsDialog
          open={open === 'ollama-models'}
          onOpenChange={(v) => !v && setOpen(null)}
        />
      )}

      {/* Copy Channel Dialog */}
      {shouldRender('copy-channel') && (
        <CopyChannelDialog
          open={open === 'copy-channel'}
          onOpenChange={(v) => !v && setOpen(null)}
        />
      )}

      {/* Multi-Key Management Dialog */}
      {shouldRender('multi-key-manage') && (
        <MultiKeyManageDialog
          open={open === 'multi-key-manage'}
          onOpenChange={(v) => !v && setOpen(null)}
        />
      )}

      {/* Tag Batch Edit Dialog */}
      {shouldRender('tag-batch-edit') && (
        <TagBatchEditDialog
          open={open === 'tag-batch-edit'}
          onOpenChange={(v) => !v && setOpen(null)}
        />
      )}

      {/* Edit Tag Dialog */}
      {shouldRender('edit-tag') && (
        <EditTagDialog
          open={open === 'edit-tag'}
          onOpenChange={(v) => !v && setOpen(null)}
        />
      )}

      {/* Upstream Model Update Dialog */}
      {shouldRender('upstream-update') && (
        <UpstreamUpdateDialog
          open={upstream.showModal}
          addModels={upstream.addModels}
          removeModels={upstream.removeModels}
          preferredTab={upstream.preferredTab}
          confirmLoading={upstream.applyLoading}
          onConfirm={upstream.applyUpdates}
          onCancel={upstream.closeModal}
        />
      )}
    </Suspense>
  )
}
