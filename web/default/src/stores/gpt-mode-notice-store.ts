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
import { create } from 'zustand'
import { persist } from 'zustand/middleware'

interface GptModeNoticeState {
  // 上次用户已确认的 gpt_mode_disabled_at 时间戳
  lastSeenDisabledAt: string
  // 弹窗是否打开（非持久化，仅当前会话）
  isOpen: boolean
  // 当前登录响应携带的 gpt_mode_disabled_at（用于用户确认时写入 lastSeenDisabledAt）
  currentDisabledAt: string
  setLastSeenDisabledAt: (at: string) => void
  setIsOpen: (open: boolean) => void
  setCurrentDisabledAt: (at: string) => void
  /**
   * 检查登录响应携带的 disabledAt 是否需要弹窗。
   * 如果 disabledAt 非空且与 lastSeenDisabledAt 不同，则打开弹窗并记录当前时间戳。
   */
  notifyIfChanged: (disabledAt: string) => void
  /**
   * 用户点击"我知道了"后调用：将 lastSeenDisabledAt 同步为 currentDisabledAt 并关闭弹窗
   */
  acknowledge: () => void
}

export const useGptModeNoticeStore = create<GptModeNoticeState>()(
  persist(
    (set, get) => ({
      lastSeenDisabledAt: '',
      isOpen: false,
      currentDisabledAt: '',

      setLastSeenDisabledAt: (at) => set({ lastSeenDisabledAt: at }),
      setIsOpen: (open) => set({ isOpen: open }),
      setCurrentDisabledAt: (at) => set({ currentDisabledAt: at }),

      notifyIfChanged: (disabledAt) => {
        if (!disabledAt) return
        const { lastSeenDisabledAt } = get()
        if (disabledAt !== lastSeenDisabledAt) {
          set({ currentDisabledAt: disabledAt, isOpen: true })
        }
      },

      acknowledge: () => {
        const { currentDisabledAt } = get()
        set({ lastSeenDisabledAt: currentDisabledAt, isOpen: false })
      },
    }),
    {
      name: 'gpt-mode-notice-storage',
      partialize: (state) => ({
        lastSeenDisabledAt: state.lastSeenDisabledAt,
      }),
    }
  )
)
