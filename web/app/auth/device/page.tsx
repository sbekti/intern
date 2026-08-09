import { DeviceCodeApproval } from "@/components/device-code-approval"
import { createPageMetadata } from "@/lib/page-titles"

export const metadata = createPageMetadata("/auth/device")

export default async function DeviceAuthPage({
  searchParams,
}: {
  searchParams: Promise<{
    user_code?: string
  }>
}) {
  const params = await searchParams

  return (
    <DeviceCodeApproval initialUserCode={params.user_code?.trim() ?? ""} />
  )
}
