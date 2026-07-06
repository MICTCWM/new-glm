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
import { useCallback, useEffect, useMemo, useState } from 'react'
import { Plus, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import type { GptGroupRow, GptGroupSettings } from '../types'

type GptGroupRatioVisualEditorProps = {
  groupRatio: Record<string, number>
  userUsableGroups: Record<string, string>
  onChange: (settings: GptGroupSettings) => void
  onDuplicateNamesChange?: (names: string[]) => void
}

const sectionCardClassName =
  'relative shadow-sm ring-0 before:pointer-events-none before:absolute before:inset-0 before:rounded-xl before:border before:border-border/90'
const sectionHeaderClassName = 'border-b bg-muted/20'

let gptGroupIdCounter = 0
function createGptGroupId() {
  gptGroupIdCounter += 1
  return `ggr_${gptGroupIdCounter}`
}

function normalizeRatio(value: unknown): number {
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : 1
}

function buildGptGroupRows(
  groupRatio: Record<string, number>,
  userUsableGroups: Record<string, string>
): GptGroupRow[] {
  const names = new Set([
    ...Object.keys(groupRatio),
    ...Object.keys(userUsableGroups),
  ])
  return Array.from(names).map((name) => ({
    _id: createGptGroupId(),
    name,
    ratio: normalizeRatio(groupRatio[name]),
    description: String(userUsableGroups[name] ?? ''),
  }))
}

function serializeGptGroupRows(rows: GptGroupRow[]): GptGroupSettings {
  const groupRatio: Record<string, number> = {}
  const userUsableGroups: Record<string, string> = {}
  for (const row of rows) {
    const name = row.name.trim()
    if (!name) continue
    groupRatio[name] = normalizeRatio(row.ratio)
    userUsableGroups[name] = row.description
  }
  return { groupRatio, userUsableGroups }
}

function gptRowsSignature(rows: GptGroupRow[]): string {
  return JSON.stringify(serializeGptGroupRows(rows))
}

function gptSourceSignature(
  groupRatio: Record<string, number>,
  userUsableGroups: Record<string, string>
): string {
  return JSON.stringify({ groupRatio, userUsableGroups })
}

export function GptGroupRatioVisualEditor(
  props: GptGroupRatioVisualEditorProps
) {
  const { t } = useTranslation()
  const [rows, setRows] = useState<GptGroupRow[]>(() =>
    buildGptGroupRows(props.groupRatio, props.userUsableGroups)
  )

  useEffect(() => {
    const incomingSignature = gptSourceSignature(
      props.groupRatio,
      props.userUsableGroups
    )
    setRows((currentRows) => {
      if (gptRowsSignature(currentRows) === incomingSignature) {
        return currentRows
      }
      return buildGptGroupRows(props.groupRatio, props.userUsableGroups)
    })
  }, [props.groupRatio, props.userUsableGroups])

  const emitRows = useCallback(
    (nextRows: GptGroupRow[]) => {
      setRows(nextRows)
      props.onChange(serializeGptGroupRows(nextRows))
    },
    [props.onChange]
  )

  const updateRow = useCallback(
    (
      id: string,
      field: Exclude<keyof GptGroupRow, '_id'>,
      value: string | number
    ) => {
      emitRows(
        rows.map((row) =>
          row._id === id ? { ...row, [field]: value } : row
        )
      )
    },
    [emitRows, rows]
  )

  const addRow = useCallback(() => {
    const existingNames = new Set(rows.map((row) => row.name))
    let index = 1
    let name = `gpt_group_${index}`
    while (existingNames.has(name)) {
      index += 1
      name = `gpt_group_${index}`
    }
    emitRows([
      ...rows,
      {
        _id: createGptGroupId(),
        name,
        ratio: 1,
        description: '',
      },
    ])
  }, [emitRows, rows])

  const removeRow = useCallback(
    (id: string) => {
      emitRows(rows.filter((row) => row._id !== id))
    },
    [emitRows, rows]
  )

  const duplicateNames = useMemo(() => {
    const counts = new Map<string, number>()
    for (const row of rows) {
      const name = row.name.trim()
      if (!name) continue
      counts.set(name, (counts.get(name) ?? 0) + 1)
    }
    return Array.from(counts.entries())
      .filter(([, count]) => count > 1)
      .map(([name]) => name)
  }, [rows])

  // 上报重复分组名给父组件，便于禁用保存按钮
  useEffect(() => {
    props.onDuplicateNamesChange?.(duplicateNames)
  }, [duplicateNames, props.onDuplicateNamesChange])

  return (
    <Card className={sectionCardClassName}>
      <CardHeader className={sectionHeaderClassName}>
        <div className='flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between'>
          <div>
            <CardTitle>{t('GPT Group Settings')}</CardTitle>
            <CardDescription>
              {t(
                'Edit GPT group ratios and user-selectable descriptions in one table.'
              )}
            </CardDescription>
          </div>
          <Button onClick={addRow} size='sm' className='sm:self-start'>
            <Plus className='mr-2 h-4 w-4' />
            {t('Add group')}
          </Button>
        </div>
      </CardHeader>
      <CardContent>
        <div className='space-y-3'>
          <div className='overflow-hidden rounded-md border'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className='min-w-40'>
                    {t('Group name')}
                  </TableHead>
                  <TableHead className='w-28'>{t('Ratio')}</TableHead>
                  <TableHead className='min-w-56'>
                    {t('Description')}
                  </TableHead>
                  <TableHead className='w-16 text-right'>
                    {t('Actions')}
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {rows.length === 0 ? (
                  <TableRow>
                    <TableCell
                      colSpan={4}
                      className='text-muted-foreground h-20 text-center text-sm'
                    >
                      {t('No groups yet. Add a group to get started.')}
                    </TableCell>
                  </TableRow>
                ) : (
                  rows.map((row) => (
                    <TableRow key={row._id}>
                      <TableCell>
                        <Input
                          value={row.name}
                          onChange={(event) =>
                            updateRow(row._id, 'name', event.target.value)
                          }
                          aria-invalid={duplicateNames.includes(
                            row.name.trim()
                          )}
                        />
                      </TableCell>
                      <TableCell>
                        <Input
                          type='number'
                          min={0}
                          step={0.1}
                          value={String(row.ratio)}
                          onChange={(event) =>
                            updateRow(
                              row._id,
                              'ratio',
                              normalizeRatio(event.target.value)
                            )
                          }
                        />
                      </TableCell>
                      <TableCell>
                        <Input
                          value={row.description}
                          placeholder={t('Group description')}
                          onChange={(event) =>
                            updateRow(
                              row._id,
                              'description',
                              event.target.value
                            )
                          }
                        />
                      </TableCell>
                      <TableCell className='text-right'>
                        <Button
                          variant='ghost'
                          size='sm'
                          onClick={() => removeRow(row._id)}
                          aria-label={t('Delete')}
                        >
                          <Trash2 className='h-4 w-4' />
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </div>

          {duplicateNames.length > 0 && (
            <p className='text-destructive text-sm'>
              {t('Duplicate group names: {{names}}', {
                names: duplicateNames.join(', '),
              })}
            </p>
          )}
        </div>
      </CardContent>
    </Card>
  )
}
