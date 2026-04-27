import { MediaGallery } from './media-gallery';
import { UploadDropzone } from './upload-dropzone';

export default function MediaPage() {
  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-2xl font-semibold">Media</h1>
      <UploadDropzone />
      <MediaGallery />
    </div>
  );
}
