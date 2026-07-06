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
import { useMemo, useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Loader2 } from 'lucide-react'
import { toast } from 'sonner'
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
import { Skeleton } from '@/components/ui/skeleton'
import { PublicLayout } from '@/components/layout'
import {
  CardStaggerContainer,
  CardStaggerItem,
  PageTransition,
} from '@/components/page-transition'
import { useIsAdmin } from '@/hooks/use-admin'
import { deleteVendor, getAllVendorMonitorSamples, getVendors } from './api'
import { VendorCard } from './components/vendor-card'
import { VendorMutateDialog } from './components/vendor-mutate-dialog'
import { VendorsPrimaryButtons } from './components/vendors-primary-buttons'
import { vendorsQueryKeys } from './lib/query-keys'
import type { Vendor, VendorMonitorSample } from './types'

export function Vendors() {
  const { t } = useTranslation()
  const isAdmin = useIsAdmin()
  const queryClient = useQueryClient()

  const [editVendor, setEditVendor] = useState<Vendor | null>(null)
  const [editOpen, setEditOpen] = useState(false)
  const [deleteVendorRow, setDeleteVendorRow] = useState<Vendor | null>(null)
  const [isDeleting, setIsDeleting] = useState(false)

  const vendorsQuery = useQuery({
    queryKey: vendorsQueryKeys.list({ page_size: 1000 }),
    queryFn: () => getVendors({ page_size: 1000 }),
  })

  // 供应商监控样本：60 秒轮询一次，staleTime 同步为 60 秒
  const monitorQuery = useQuery({
    queryKey: ['vendor-monitor-samples-all'],
    queryFn: getAllVendorMonitorSamples,
    staleTime: 60 * 1000,
    refetchInterval: 60 * 1000,
    retry: false,
  })

  const vendors = vendorsQuery.data?.data?.items ?? []

  // 按 vendor_id 分发监控样本到各卡片
  const monitorMap = useMemo(() => {
    const map = new Map<number, VendorMonitorSample[]>()
    const raw = monitorQuery.data
    if (!raw) return map
    for (const [vendorIdStr, samples] of Object.entries(raw)) {
      const vendorId = Number(vendorIdStr)
      if (!Number.isNaN(vendorId)) {
        map.set(vendorId, samples)
      }
    }
    return map
  }, [monitorQuery.data])

  const handleEdit = (vendor: Vendor) => {
    setEditVendor(vendor)
    setEditOpen(true)
  }

  const handleDelete = (vendor: Vendor) => {
    setDeleteVendorRow(vendor)
  }

  const confirmDelete = async () => {
    if (!deleteVendorRow) return
    setIsDeleting(true)
    try {
      const response = await deleteVendor(deleteVendorRow.id)
      if (response.success) {
        toast.success(t('Vendor deleted successfully'))
        queryClient.invalidateQueries({ queryKey: vendorsQueryKeys.lists() })
        setDeleteVendorRow(null)
      } else {
        toast.error(response.message || t('Failed to delete vendor'))
      }
    } catch (error: unknown) {
      toast.error(
        (error as Error)?.message || t('Failed to delete vendor')
      )
    } finally {
      setIsDeleting(false)
    }
  }

  return (
    <PublicLayout showMainContainer={false}>
      <PageTransition className='mx-auto w-full max-w-[1280px] space-y-8 px-3 pt-16 pb-10 sm:px-6 sm:pt-20 sm:pb-12 xl:px-8'>
        <header className='flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between'>
          <div className='space-y-2'>
            <h1 className='text-foreground text-3xl font-bold tracking-tight sm:text-4xl'>
              {t('Vendors')}
            </h1>
            <p className='text-muted-foreground text-sm sm:text-base'>
              {t('Vendors list and management')}
            </p>
          </div>
          <VendorsPrimaryButtons />
        </header>

        {vendorsQuery.isLoading ? (
          <VendorsLoading />
        ) : vendorsQuery.isError ? (
          <VendorsError
            message={
              vendorsQuery.error instanceof Error
                ? vendorsQuery.error.message
                : t('Unable to load vendors data')
            }
          />
        ) : vendors.length === 0 ? (
          <VendorsEmpty />
        ) : (
          <CardStaggerContainer className='grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3'>
            {vendors.map((vendor) => (
              <CardStaggerItem key={vendor.id} className='h-full'>
                <VendorCard
                  vendor={vendor}
                  onEdit={handleEdit}
                  onDelete={handleDelete}
                  canManage={isAdmin}
                  monitorSamples={monitorMap.get(vendor.id)}
                />
              </CardStaggerItem>
            ))}
          </CardStaggerContainer>
        )}
      </PageTransition>

      <VendorMutateDialog
        open={editOpen}
        onOpenChange={(open) => {
          setEditOpen(open)
          if (!open) setEditVendor(null)
        }}
        currentVendor={editVendor}
      />

      <AlertDialog
        open={deleteVendorRow !== null}
        onOpenChange={(open) => {
          if (!open) setDeleteVendorRow(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t('Delete Vendor')}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t('Are you sure you want to delete vendor "{{name}}"?', {
                name: deleteVendorRow?.name,
              })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={isDeleting}>
              {t('Cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              onClick={confirmDelete}
              disabled={isDeleting}
              className='bg-destructive text-destructive-foreground hover:bg-destructive/90'
            >
              {isDeleting && (
                <Loader2 className='mr-2 h-4 w-4 animate-spin' />
              )}
              {t('Delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </PublicLayout>
  )
}

function VendorsLoading() {
  return (
    <div className='grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3'>
      {Array.from({ length: 6 }).map((_, index) => (
        <Skeleton key={index} className='h-[140px] w-full rounded-xl' />
      ))}
    </div>
  )
}

function VendorsError(props: { message: string }) {
  const { t } = useTranslation()
  return (
    <div className='bg-card rounded-xl border border-dashed px-6 py-12 text-center'>
      <h2 className='text-foreground text-base font-semibold'>
        {t('Unable to load vendors')}
      </h2>
      <p className='text-muted-foreground mx-auto mt-2 max-w-md text-sm'>
        {props.message}
      </p>
    </div>
  )
}

function VendorsEmpty() {
  const { t } = useTranslation()
  return (
    <div className='bg-card rounded-xl border border-dashed px-6 py-12 text-center'>
      <h2 className='text-foreground text-base font-semibold'>
        {t('No vendors yet')}
      </h2>
      <p className='text-muted-foreground mx-auto mt-2 max-w-md text-sm'>
        {t('Vendors will appear here once they are added.')}
      </p>
    </div>
  )
}
