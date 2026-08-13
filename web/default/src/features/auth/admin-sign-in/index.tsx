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
import type { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Loader2, LogIn, ShieldCheck } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { PasswordInput } from '@/components/password-input'
import {
  InputOTP,
  InputOTPGroup,
  InputOTPSlot,
  InputOTPSeparator,
} from '@/components/ui/input-otp'
import { loginAdmin, login2fa } from '@/features/auth/api'
import { loginFormSchema, OTP_LENGTH } from '@/features/auth/constants'
import { useAuthRedirect } from '@/features/auth/hooks/use-auth-redirect'
import { AuthLayout } from '@/features/auth/auth-layout'

/**
 * 管理员专属登录页（不受区域限制拦截）。
 * 仅允许管理员/超级管理员账号登录，普通用户账号会被后端拒绝。
 * 启用 2FA 的管理员可在页面内直接输入验证码完成登录。
 */
export function AdminSignIn() {
  const { t } = useTranslation()
  const [isLoading, setIsLoading] = useState(false)
  const [is2FA, setIs2FA] = useState(false)
  const [otp, setOtp] = useState('')
  const { handleLoginSuccess } = useAuthRedirect()

  const form = useForm<z.infer<typeof loginFormSchema>>({
    resolver: zodResolver(loginFormSchema),
    defaultValues: {
      username: '',
      password: '',
    },
  })

  async function onSubmit(data: z.infer<typeof loginFormSchema>) {
    setIsLoading(true)
    try {
      const res = await loginAdmin({
        username: data.username,
        password: data.password,
      })

      if (res.success) {
        if (res.data?.require_2fa) {
          setIs2FA(true)
          return
        }
        await handleLoginSuccess(res.data as { id?: number } | null)
        toast.success(t('Welcome back!'))
      }
    } catch (_error) {
      // Errors are handled by global interceptor
    } finally {
      setIsLoading(false)
    }
  }

  async function handleVerify2FA() {
    if (otp.length < OTP_LENGTH) {
      return
    }
    setIsLoading(true)
    try {
      const res = await login2fa({ code: otp })
      if (!res.success) {
        toast.error(res.message || t('Invalid code'))
        return
      }
      if (!res.data) {
        throw new Error('No user data received from login')
      }
      await handleLoginSuccess(res.data as { id?: number } | null)
      toast.success(t('Welcome back!'))
    } catch (_error) {
      // Errors are handled by global interceptor
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <AuthLayout>
      <div className='w-full space-y-8'>
        <div className='space-y-2'>
          <h2 className='flex items-center justify-center gap-2 text-center text-2xl font-semibold tracking-tight sm:justify-start'>
            <ShieldCheck className='h-6 w-6' />
            {t('Admin Login')}
          </h2>
          <p className='text-muted-foreground text-center text-sm sm:text-left'>
            {t('Admin Only')}
          </p>
        </div>

        {is2FA ? (
          <Form {...form}>
            <form
              onSubmit={(e) => {
                e.preventDefault()
                handleVerify2FA()
              }}
              className='grid gap-4'
            >
              <FormItem>
                <FormLabel>{t('Verification Code')}</FormLabel>
                <FormControl>
                  <InputOTP
                    maxLength={OTP_LENGTH}
                    value={otp}
                    onChange={setOtp}
                    containerClassName='justify-between sm:[&>[data-slot="input-otp-group"]>div]:w-12'
                  >
                    <InputOTPGroup>
                      <InputOTPSlot index={0} />
                      <InputOTPSlot index={1} />
                    </InputOTPGroup>
                    <InputOTPSeparator />
                    <InputOTPGroup>
                      <InputOTPSlot index={2} />
                      <InputOTPSlot index={3} />
                    </InputOTPGroup>
                    <InputOTPSeparator />
                    <InputOTPGroup>
                      <InputOTPSlot index={4} />
                      <InputOTPSlot index={5} />
                    </InputOTPGroup>
                  </InputOTP>
                </FormControl>
                <FormMessage />
              </FormItem>
              <Button
                type='submit'
                className='mt-2 w-full justify-center gap-2'
                disabled={isLoading || otp.length < OTP_LENGTH}
              >
                {isLoading ? <Loader2 className='animate-spin' /> : null}
                {t('Verify and Sign In')}
              </Button>
            </form>
          </Form>
        ) : (
          <Form {...form}>
            <form
              onSubmit={form.handleSubmit(onSubmit)}
              className='grid gap-4'
            >
              <FormField
                control={form.control}
                name='username'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Username or Email')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder={t('Enter your username or email')}
                        {...field}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='password'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Password')}</FormLabel>
                    <FormControl>
                      <PasswordInput
                        placeholder={t('Enter password')}
                        {...field}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <Button
                type='submit'
                className='mt-2 w-full justify-center gap-2'
                disabled={isLoading}
              >
                {isLoading ? (
                  <Loader2 className='animate-spin' />
                ) : (
                  <LogIn />
                )}
                {t('Sign in')}
              </Button>
            </form>
          </Form>
        )}
      </div>
    </AuthLayout>
  )
}