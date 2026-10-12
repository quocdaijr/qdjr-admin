import { PageHeader } from '@/components/page-header';

import { MediaGallery } from './media-gallery';
import { UploadDropzone } from './upload-dropzone';

export default function MediaPage() {
  return (
    <>
      <PageHeader title="Media" />
      <UploadDropzone />
      <MediaGallery />
    </>
  );
}
