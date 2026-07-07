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
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { useGptModeNoticeStore } from '@/stores/gpt-mode-notice-store'

/**
 * 登录后弹窗组件：当管理员关闭 GPT 模式后，告知用户其 GPT 模式已被关闭、额度已转为基础额度。
 * 弹窗状态由 gpt-mode-notice-store 管理；登录流程在检测到 gpt_mode_disabled=true 时
 * 调用 notifyIfChanged(gpt_mode_disabled_at) 触发弹窗。
 */
export function GptModeDisabledDialog() {
  const { t } = useTranslation()
  const isOpen = useGptModeNoticeStore((s) => s.isOpen)
  const acknowledge = useGptModeNoticeStore((s) => s.acknowledge)
  const setIsOpen = useGptModeNoticeStore((s) => s.setIsOpen)

  return (
    <Dialog
      open={isOpen}
      onOpenChange={(open) => {
        // 仅允许通过按钮关闭，避免点击遮罩意外关闭后丢失"已确认"标记
        if (!open) return
        setIsOpen(open)
      }}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('GPT Mode Disabled')}</DialogTitle>
          <DialogDescription>
            {t(
              'The administrator has disabled GPT mode. Your GPT mode has been turned off and your GPT quota has been automatically converted to base quota.'
            )}
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button onClick={acknowledge}>{t('I understand')}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
