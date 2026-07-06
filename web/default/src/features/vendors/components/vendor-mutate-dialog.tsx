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
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useQueryClient } from '@tanstack/react-query'
import { Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
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
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Textarea } from '@/components/ui/textarea'
import { createVendor, updateVendor } from '../api'
import { vendorsQueryKeys } from '../lib/query-keys'
import {
  SUPPLY_TYPE,
  vendorFormSchema,
  type Vendor,
  type VendorFormValues,
} from '../types'

type VendorMutateDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentVendor?: Vendor | null
}

export function VendorMutateDialog({
  open,
  onOpenChange,
  currentVendor,
}: VendorMutateDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const isEdit = Boolean(currentVendor?.id)
  const [isSaving, setIsSaving] = useState(false)

  const form = useForm({
    resolver: zodResolver(vendorFormSchema),
    defaultValues: {
      name: '',
      description: '',
      icon: '',
      status: 1,
      supply_type: SUPPLY_TYPE.SELF,
    },
  })

  // Load vendor data for editing
  useEffect(() => {
    if (open && isEdit && currentVendor) {
      form.reset({
        id: currentVendor.id,
        name: currentVendor.name,
        description: currentVendor.description || '',
        icon: currentVendor.icon || '',
        status: currentVendor.status || 1,
        supply_type:
          currentVendor.supply_type === SUPPLY_TYPE.PARTNER
            ? SUPPLY_TYPE.PARTNER
            : SUPPLY_TYPE.SELF,
      })
    } else if (open && !isEdit) {
      form.reset({
        name: '',
        description: '',
        icon: '',
        status: 1,
        supply_type: SUPPLY_TYPE.SELF,
      })
    }
  }, [open, isEdit, currentVendor, form])

  const onSubmit = async (values: VendorFormValues) => {
    setIsSaving(true)
    try {
      const response = isEdit
        ? await updateVendor({ ...values, id: currentVendor!.id })
        : await createVendor(values)

      if (response.success) {
        toast.success(
          isEdit
            ? t('Vendor updated successfully')
            : t('Vendor created successfully')
        )
        queryClient.invalidateQueries({ queryKey: vendorsQueryKeys.lists() })
        onOpenChange(false)
      } else {
        toast.error(response.message || t('Operation failed'))
      }
    } catch (error: unknown) {
      toast.error((error as Error)?.message || t('Operation failed'))
    } finally {
      setIsSaving(false)
    }
  }

  const supplyTypeOptions = [
    {
      value: String(SUPPLY_TYPE.SELF),
      id: `supply-${SUPPLY_TYPE.SELF}`,
      label: t('Self-supplied'),
      description: t('Self-supplied vendor, non-third-party, more stable'),
    },
    {
      value: String(SUPPLY_TYPE.PARTNER),
      id: `supply-${SUPPLY_TYPE.PARTNER}`,
      label: t('Partner-supplied'),
      description: t('Third-party partner vendor, stable and reliable'),
    },
  ]

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>
            {isEdit ? t('Edit Vendor') : t('Add Vendor')}
          </DialogTitle>
          <DialogDescription>
            {isEdit
              ? t('Update vendor information for {{name}}', {
                  name: currentVendor?.name,
                })
              : t('Add a new vendor to the system')}
          </DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-4'>
            <FormField
              control={form.control}
              name='name'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Vendor name')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder={t('OpenAI, Anthropic, etc.')}
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('The unique name for this vendor')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='description'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Vendor description')}</FormLabel>
                  <FormControl>
                    <Textarea
                      placeholder={t('Describe this vendor...')}
                      rows={3}
                      {...field}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='icon'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Vendor icon')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder={t('OpenAI, Anthropic, Google, etc.')}
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('@lobehub/icons key name')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='supply_type'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Supply Type')}</FormLabel>
                  <FormControl>
                    <RadioGroup
                      value={String(field.value ?? SUPPLY_TYPE.SELF)}
                      onValueChange={(value) =>
                        field.onChange(parseInt(value, 10))
                      }
                      className='grid grid-cols-1 gap-3 sm:grid-cols-2'
                    >
                      {supplyTypeOptions.map((option) => (
                        <div
                          key={option.id}
                          className='flex items-start space-x-2 rounded-lg border p-3'
                        >
                          <RadioGroupItem
                            value={option.value}
                            id={option.id}
                            className='mt-0.5'
                          />
                          <div className='space-y-0.5'>
                            <Label
                              htmlFor={option.id}
                              className='cursor-pointer font-medium'
                            >
                              {option.label}
                            </Label>
                            <p className='text-muted-foreground text-xs'>
                              {option.description}
                            </p>
                          </div>
                        </div>
                      ))}
                    </RadioGroup>
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <DialogFooter>
              <Button
                type='button'
                variant='outline'
                onClick={() => onOpenChange(false)}
                disabled={isSaving}
              >
                {t('Cancel')}
              </Button>
              <Button type='submit' disabled={isSaving}>
                {isSaving && <Loader2 className='mr-2 h-4 w-4 animate-spin' />}
                {isSaving
                  ? t('Saving...')
                  : isEdit
                    ? t('Update')
                    : t('Create')}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}
